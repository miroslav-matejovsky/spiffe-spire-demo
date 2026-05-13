$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot

if (-not (Get-Command podman-compose -ErrorAction SilentlyContinue)) {
    throw "podman-compose was not found on PATH. Install it and try again."
}

Push-Location $scenarioRoot
try {
    Write-Host "Stopping TPM learning scenario..." -ForegroundColor Cyan
    podman-compose down -v --remove-orphans *> $null
    Write-Host "TPM learning scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
