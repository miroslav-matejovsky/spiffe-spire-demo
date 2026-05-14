[CmdletBinding()]
param()

# Set up the SPIRE metrics scenario
# Starts all containers and waits for services to be ready

$ErrorActionPreference = "Stop"

$scenarioRoot    = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$scenarioName    = Split-Path -Leaf $scenarioRoot
$provisionScript = Join-Path $PSScriptRoot "provision-agent.ps1"
$agentKeyPath    = Join-Path $scenarioRoot "spire" "agent" "agent.key.pem"
$agentCertPath   = Join-Path $scenarioRoot "spire" "agent" "agent.crt.pem"
$caCertPath      = Join-Path $scenarioRoot "spire" "server" "agent-cacert.pem"
$repoRoot        = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

Push-Location $scenarioRoot
try {
    Write-Step "Starting metrics scenario..."

    if (-not (Test-Path $agentKeyPath) -or -not (Test-Path $agentCertPath) -or -not (Test-Path $caCertPath)) {
        Write-Warn "Agent credentials not found. Running provisioning first..."
        & $provisionScript
    }

    Write-Step "Starting all containers..."
    podman-compose up -d

    Write-Step "Waiting for services to start..."
    $required = @("prometheus", "graphite", "metrics-spire-server", "metrics-spire-agent")
    $missing  = @()

    for ($attempt = 1; $attempt -le 10; $attempt++) {
        Start-Sleep -Seconds 3
        $containers = @(podman ps --format "{{.Names}}")
        Write-Detail "attempt $attempt/10: running containers: $($containers -join ', ')"
        $missing = $required | Where-Object { -not ($containers -match $_) }
        if ($missing.Count -eq 0) { break }
    }

    foreach ($name in $required) {
        $containers = @(podman ps --format "{{.Names}}")
        if ($containers -match $name) {
            Write-Ok "$name is running"
        }
        else {
            Write-Host "   ❌ $name is NOT running" -ForegroundColor Red
        }
    }

    if ($missing.Count -gt 0) {
        throw "Required container(s) did not start: $($missing -join ', ')"
    }

    Write-Ok "Metrics scenario is ready!"
    Write-Info "Prometheus: http://localhost:9090"
    Write-Info "Graphite:   http://localhost:8080"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs "metrics-spire-server"
    Show-ContainerLogs "metrics-spire-agent"
    Show-ContainerLogs "prometheus"
    Show-ContainerLogs "graphite"
    Show-PodmanStatus
    throw
}
finally {
    Pop-Location
}
