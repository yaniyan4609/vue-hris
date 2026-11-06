$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'launcher-common.ps1')
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$stateFile = Join-Path $repoRoot 'work/launcher/current.json'
if (-not (Test-Path -LiteralPath $stateFile)) {
    Write-Host 'Tidak ada instance HRIS launcher yang tercatat.'
    return
}
$state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
Stop-HrisOwnedProcess -ProcessId $state.frontendPid -StartedUtcTicks $state.frontendStartedUtcTicks
Stop-HrisOwnedProcess -ProcessId $state.backendPid -StartedUtcTicks $state.backendStartedUtcTicks
Remove-Item -LiteralPath $stateFile -Force
Write-Host 'HRIS launcher dihentikan. File log tetap tersimpan.' -ForegroundColor Green
