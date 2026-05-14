$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Stopping workload scenario..." -ForegroundColor Cyan
    podman-compose down -v --remove-orphans *>$null
    Write-Host "Workload scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
