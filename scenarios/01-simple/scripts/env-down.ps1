$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$scenarioName = Split-Path -Leaf $scenarioRoot
$agentContainerName = "${scenarioName}_spire-agent_1"

Push-Location $scenarioRoot
try {
    Write-Host "Stopping simple SPIRE scenario..." -ForegroundColor Cyan

    $existing = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existing -contains $agentContainerName) {
        podman rm -f $agentContainerName *>$null
    }

    podman-compose down *>$null
    Write-Host "Simple scenario stopped." -ForegroundColor Green
} finally {
    Pop-Location
}
