$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

if (-not (Get-Command podman-compose -ErrorAction SilentlyContinue)) {
    throw "podman-compose was not found on PATH. Install it and try again."
}

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Stopping metrics scenario..." -ForegroundColor Cyan
    podman-compose down -v --remove-orphans
    Write-Host "Metrics scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
