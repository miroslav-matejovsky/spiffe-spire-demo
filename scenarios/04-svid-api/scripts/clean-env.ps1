$ErrorActionPreference = "Stop"

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Stopping SVID API scenario..." -ForegroundColor Cyan
    podman-compose down -v
    Write-Host "SVID API scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
