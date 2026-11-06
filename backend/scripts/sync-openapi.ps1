$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $repoRoot 'docs/openapi.yaml'
$destination = Join-Path $repoRoot 'backend/internal/httpapi/apidocs/openapi.yaml'
Copy-Item -LiteralPath $source -Destination $destination -Force
Write-Output 'OpenAPI source copied into the Go API docs bundle.'
