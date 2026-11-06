function Get-HrisOwnedProcess {
    param([int]$ProcessId, [string]$StartedUtcTicks)
    if ($ProcessId -le 0 -or -not $StartedUtcTicks) { return $null }
    try {
        $process = Get-Process -Id $ProcessId -ErrorAction Stop
        if ($process.StartTime.ToUniversalTime().Ticks.ToString() -eq $StartedUtcTicks) { return $process }
    } catch { return $null }
    return $null
}

function Stop-HrisOwnedProcess {
    param([int]$ProcessId, [string]$StartedUtcTicks)
    $process = Get-HrisOwnedProcess -ProcessId $ProcessId -StartedUtcTicks $StartedUtcTicks
    if ($process) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Write-Host "Proses $($process.Id) dihentikan."
    }
}

function Test-HrisPortAvailable {
    param([int]$Port)
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Any, $Port)
    try {
        $listener.Start()
        return $true
    } catch {
        return $false
    } finally {
        $listener.Stop()
    }
}

function Wait-HrisURL {
    param([string]$Url, [int]$Seconds, [System.Diagnostics.Process]$Process)
    $deadline = (Get-Date).AddSeconds($Seconds)
    while ((Get-Date) -lt $deadline) {
        if ($Process.HasExited) { return $false }
        try {
            $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
            if ($response.StatusCode -eq 200) { return $true }
        } catch { }
        Start-Sleep -Milliseconds 700
    }
    return $false
}
