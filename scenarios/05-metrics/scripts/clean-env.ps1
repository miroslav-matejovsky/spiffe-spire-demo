# Clean up the SPIRE metrics scenario
# Stops and removes all containers

$ErrorActionPreference = "Stop"

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Stopping metrics scenario..." -ForegroundColor Cyan

    podman-compose down

    Write-Host "Metrics scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
