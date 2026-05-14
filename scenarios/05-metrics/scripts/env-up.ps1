# Set up the SPIRE metrics scenario
# Starts all containers and waits for services to be ready

$ErrorActionPreference = "Stop"

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$provisionScript = Join-Path $PSScriptRoot "provision-agent.ps1"
$agentKeyPath = Join-Path $ScenarioRoot "spire" "agent" "agent.key.pem"
$agentCertPath = Join-Path $ScenarioRoot "spire" "agent" "agent.crt.pem"
$caCertPath = Join-Path $ScenarioRoot "spire" "server" "agent-cacert.pem"
Push-Location $ScenarioRoot

try {
    Write-Host "Starting metrics scenario..." -ForegroundColor Cyan

    if (-not (Test-Path $agentKeyPath) -or -not (Test-Path $agentCertPath) -or -not (Test-Path $caCertPath)) {
        Write-Host "Agent credentials not found. Running provisioning first..." -ForegroundColor Yellow
        & $provisionScript
    }

    # Start the compose stack
    podman-compose up -d

    # Wait for services to start
    Write-Host "Waiting for services to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10

    # Verify containers are running
    $containers = podman ps --format "{{.Names}}"
    $required = @("prometheus", "graphite")

    foreach ($name in $required) {
        if ($containers -match $name) {
            Write-Host "  ✅ $name is running" -ForegroundColor Green
        } else {
            Write-Host "  ❌ $name is NOT running" -ForegroundColor Red
        }
    }

    Write-Host "`nMetrics scenario is ready!" -ForegroundColor Green
    Write-Host "  Prometheus: http://localhost:9090" -ForegroundColor Gray
    Write-Host "  Graphite:   http://localhost:8080" -ForegroundColor Gray
}
finally {
    Pop-Location
}
