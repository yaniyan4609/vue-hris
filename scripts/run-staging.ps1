$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'launcher-common.ps1')

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$backendDir = Join-Path $repoRoot 'backend'
$frontendDir = Join-Path $repoRoot 'frontend'
$workDir = Join-Path $repoRoot 'work/launcher'
$stateFile = Join-Path $workDir 'current.json'
$credentialFile = Join-Path $workDir 'db-credential.xml'
$backendProcess = $null
$frontendProcess = $null
$state = $null
$ready = $false
New-Item -ItemType Directory -Force -Path $workDir | Out-Null

try {
    if (Test-Path -LiteralPath $stateFile) {
        $existing = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
        $existingBackend = Get-HrisOwnedProcess -ProcessId $existing.backendPid -StartedUtcTicks $existing.backendStartedUtcTicks
        $existingFrontend = Get-HrisOwnedProcess -ProcessId $existing.frontendPid -StartedUtcTicks $existing.frontendStartedUtcTicks
        if ($existingBackend -and $existingFrontend) {
            Write-Host 'HRIS sudah berjalan.' -ForegroundColor Green
            Write-Host "Frontend : $($existing.frontendUrl)"
            Write-Host "Backend  : $($existing.backendUrl)"
            Write-Host "WebSocket: $($existing.websocketUrl)"
            Write-Host "API Docs : $($existing.docsUrl)"
            Start-Process $existing.frontendUrl
            Start-Process $existing.docsUrl
            return
        }
        Remove-Item -LiteralPath $stateFile -Force
    }

    $go = (Get-Command go.exe -ErrorAction Stop).Source
    $node = (Get-Command node.exe -ErrorAction Stop).Source
    $vite = Join-Path $frontendDir 'node_modules/vite/bin/vite.js'
    if (-not (Test-Path -LiteralPath $vite)) {
        Write-Host 'Memasang dependency frontend...'
        Push-Location $frontendDir
        try {
            & npm.cmd ci
            if ($LASTEXITCODE -ne 0) { throw 'npm ci gagal.' }
        } finally { Pop-Location }
    }

    $ipConfigs = @(Get-NetIPConfiguration | Where-Object { $_.IPv4Address -and $_.IPv4DefaultGateway })
    $localAddresses = @(Get-NetIPAddress -AddressFamily IPv4 | Where-Object { $_.AddressState -eq 'Preferred' } | ForEach-Object { $_.IPAddress })
    if ($env:HRIS_LAN_IP) {
        if ($localAddresses -notcontains $env:HRIS_LAN_IP) { throw "HRIS_LAN_IP $($env:HRIS_LAN_IP) bukan alamat IPv4 komputer ini." }
        $lanIP = $env:HRIS_LAN_IP
    } elseif ($ipConfigs.Count -gt 0) {
        $lanIP = $ipConfigs[0].IPv4Address[0].IPAddress
    } else {
        throw 'IP LAN tidak ditemukan. Sambungkan Ethernet/Wi-Fi atau isi HRIS_LAN_IP dengan IP komputer ini.'
    }

    $portPairs = @(@(38473, 38474), @(39473, 39474), @(40473, 40474), @(41473, 41474))
    $ports = $null
    foreach ($pair in $portPairs) {
        if ((Test-HrisPortAvailable $pair[0]) -and (Test-HrisPortAvailable $pair[1])) { $ports = $pair; break }
    }
    if (-not $ports) { throw 'Semua pasangan port HRIS sedang dipakai. Hentikan instance lama atau proses lain lalu coba lagi.' }
    $frontendPort = $ports[0]
    $backendPort = $ports[1]
    $frontendUrl = "http://${lanIP}:${frontendPort}/"
    $backendUrl = "http://${lanIP}:${backendPort}/"
    $docsUrl = "${frontendUrl}api/v1/docs/"
    $websocketUrl = "ws://${lanIP}:${frontendPort}/api/v1/realtime/ws"

    $instanceDir = Join-Path $workDir ([DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss') + '-' + [Guid]::NewGuid().ToString('N').Substring(0, 6))
    New-Item -ItemType Directory -Force -Path $instanceDir | Out-Null
    $backendExe = Join-Path $instanceDir 'hris-api.exe'
    Write-Host 'Membangun backend...'
    Push-Location $backendDir
    try {
        & $go build -o $backendExe ./cmd/api
        if ($LASTEXITCODE -ne 0) { throw 'Build backend gagal.' }
    } finally { Pop-Location }

    $dbUser = 'diserve'
    $securePassword = $null
    $newCredential = $false
    if (-not $env:DB_PASSWORD) {
        if (Test-Path -LiteralPath $credentialFile) {
            try {
                $credential = Import-Clixml -LiteralPath $credentialFile
                if ($credential -isnot [System.Management.Automation.PSCredential] -or $credential.UserName -ne $dbUser) { throw 'Credential tidak sesuai.' }
                $securePassword = $credential.Password
            } catch {
                Write-Host 'Credential tersimpan tidak dapat dibaca; masukkan ulang kata sandi.' -ForegroundColor Yellow
            }
        }
        if (-not $securePassword) {
            $securePassword = Read-Host 'Kata sandi MariaDB staging (disimpan terenkripsi untuk akun Windows ini)' -AsSecureString
            $newCredential = $true
        }
        $env:DB_PASSWORD = ([System.Net.NetworkCredential]::new('', $securePassword)).Password
    }
    if (-not $env:DB_PASSWORD) { throw 'Kata sandi database kosong.' }

    $env:DB_ADDR = '192.168.11.85:3306'
    $env:DB_USER = $dbUser
    $env:DB_NAME = 'staging-saas-hris'
    $env:APP_ENV = 'staging'
    $env:ENABLE_DEMO_LOGIN = 'true'
    $env:COOKIE_SECURE = 'false'
    $env:APP_ORIGIN = "http://${lanIP}:${frontendPort}"
    $env:HTTP_ADDR = "0.0.0.0:${backendPort}"
    $backendProcess = Start-Process -FilePath $backendExe -WorkingDirectory $backendDir -WindowStyle Hidden -RedirectStandardOutput (Join-Path $instanceDir 'backend.out.log') -RedirectStandardError (Join-Path $instanceDir 'backend.err.log') -PassThru
    Remove-Item Env:DB_PASSWORD -ErrorAction SilentlyContinue
    if (-not (Wait-HrisURL -Url "http://127.0.0.1:${backendPort}/readyz" -Seconds 30 -Process $backendProcess)) {
        throw "Backend gagal siap. Periksa $(Join-Path $instanceDir 'backend.err.log')"
    }
    if ($newCredential) {
        [System.Management.Automation.PSCredential]::new($dbUser, $securePassword) | Export-Clixml -LiteralPath $credentialFile
    }
    $securePassword = $null

    $env:VITE_API_PROXY = "http://127.0.0.1:${backendPort}"
    $frontendProcess = Start-Process -FilePath $node -ArgumentList @("`"$vite`"", '--host', '0.0.0.0', '--port', "$frontendPort", '--strictPort') -WorkingDirectory $frontendDir -WindowStyle Hidden -RedirectStandardOutput (Join-Path $instanceDir 'frontend.out.log') -RedirectStandardError (Join-Path $instanceDir 'frontend.err.log') -PassThru
    if (-not (Wait-HrisURL -Url "http://127.0.0.1:${frontendPort}/api/v1/docs/" -Seconds 30 -Process $frontendProcess)) {
        throw "Frontend gagal siap. Periksa $(Join-Path $instanceDir 'frontend.err.log')"
    }

    $state = [ordered]@{
        backendPid = $backendProcess.Id
        backendStartedUtcTicks = $backendProcess.StartTime.ToUniversalTime().Ticks.ToString()
        frontendPid = $frontendProcess.Id
        frontendStartedUtcTicks = $frontendProcess.StartTime.ToUniversalTime().Ticks.ToString()
        frontendUrl = $frontendUrl
        backendUrl = $backendUrl
        websocketUrl = $websocketUrl
        docsUrl = $docsUrl
        logsDir = $instanceDir
    }
    $state | ConvertTo-Json | Set-Content -LiteralPath $stateFile -Encoding UTF8
    $ready = $true
    Write-Host ''
    Write-Host 'HRIS staging siap.' -ForegroundColor Green
    Write-Host "Frontend : $frontendUrl"
    Write-Host "Backend  : $backendUrl"
    Write-Host "WebSocket: $websocketUrl"
    Write-Host "API Docs : $docsUrl"
    Write-Host "Log      : $instanceDir"
    Write-Host ''
    Start-Process $frontendUrl
    Start-Process $docsUrl
    Read-Host 'Tekan Enter untuk menghentikan frontend dan backend' | Out-Null
} catch {
    Write-Host "Gagal menjalankan HRIS: $($_.Exception.Message)" -ForegroundColor Red
    throw
} finally {
    Remove-Item Env:DB_PASSWORD -ErrorAction SilentlyContinue
    if ($ready -and $state) {
        Stop-HrisOwnedProcess -ProcessId $state.frontendPid -StartedUtcTicks $state.frontendStartedUtcTicks
        Stop-HrisOwnedProcess -ProcessId $state.backendPid -StartedUtcTicks $state.backendStartedUtcTicks
        if (Test-Path -LiteralPath $stateFile) {
            $current = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
            if ($current.backendPid -eq $state.backendPid -and $current.backendStartedUtcTicks -eq $state.backendStartedUtcTicks) {
                Remove-Item -LiteralPath $stateFile -Force
            }
        }
    } elseif (-not $ready) {
        if ($frontendProcess) { Stop-Process -Id $frontendProcess.Id -Force -ErrorAction SilentlyContinue }
        if ($backendProcess) { Stop-Process -Id $backendProcess.Id -Force -ErrorAction SilentlyContinue }
    }
}
