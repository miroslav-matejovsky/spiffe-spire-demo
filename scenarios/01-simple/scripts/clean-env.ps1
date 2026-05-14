$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot

Push-Location $scenarioRoot
try {
    Write-Host "Stopping simple SPIRE scenario..." -ForegroundColor Cyan
    podman-compose down | Out-Null
    Write-Host "Simple scenario stopped." -ForegroundColor Green
} finally {
    Pop-Location
}
