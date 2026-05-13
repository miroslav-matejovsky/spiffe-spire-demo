$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$agentContainerName = "spire-simple-agent"

Push-Location $scenarioRoot
try {
    Write-Host "Stopping simple SPIRE scenario..." -ForegroundColor Cyan

    $existingContainers = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existingContainers -contains $agentContainerName) {
        podman rm -f $agentContainerName | Out-Null
    }

    podman-compose down 2>&1 | Out-Null

    Write-Host "Simple scenario stopped." -ForegroundColor Green
} finally {
    Pop-Location
}
