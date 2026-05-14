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

$maxAttempts = 15

Push-Location $scenarioRoot
try {
    Write-Step "Starting metrics scenario..."

    if (-not (Test-Path $agentKeyPath) -or -not (Test-Path $agentCertPath) -or -not (Test-Path $caCertPath)) {
        Write-Warn "Agent credentials not found. Running provisioning first..."
        & $provisionScript
    }

    Write-Step "Starting core services..."
    podman-compose up -d graphite-statsd prometheus spire-server spire-agent *>$null

    Write-Step "Waiting for services to start..."
    $requiredCore = @("prometheus", "graphite", "metrics-spire-server", "metrics-spire-agent")
    $missing  = @()

    for ($attempt = 1; $attempt -le 10; $attempt++) {
        Start-Sleep -Seconds 3
        $containers = @(podman ps --format "{{.Names}}")
        Write-Detail "attempt $attempt/10: running containers: $($containers -join ', ')"
        $missing = $requiredCore | Where-Object { -not ($containers -match $_) }
        if ($missing.Count -eq 0) { break }
    }

    foreach ($name in $requiredCore) {
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

    Write-Step "Starting Tornjak..."
    podman-compose up -d tornjak-backend tornjak-frontend *>$null

    $required = $requiredCore + @("metrics-tornjak-backend", "metrics-tornjak-frontend")
    for ($attempt = 1; $attempt -le 10; $attempt++) {
        Start-Sleep -Seconds 2
        $containers = @(podman ps --format "{{.Names}}")
        $missing = $required | Where-Object { -not ($containers -match $_) }
        if ($missing.Count -eq 0) { break }
    }
    if ($missing.Count -gt 0) {
        throw "Required container(s) did not start: $($missing -join ', ')"
    }

    Write-Step "Waiting for Tornjak backend to become ready..."
    $tornjakReady = $false
    for ($attempt = 1; $attempt -le  30; $attempt++) {
        try {
            $response = Invoke-WebRequest -Uri "http://127.0.0.1:10000/api/tornjak/serverinfo" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -lt 500) {
                $tornjakReady = $true
                break
            }
        } catch {}
        Write-Detail "attempt $attempt/${maxAttempts}: waiting for Tornjak backend..."
        Start-Sleep -Seconds 2
    }

    if (-not $tornjakReady) {
        Write-Warn "Tornjak backend did not respond in time — it may still be starting."
    } else {
        Write-Ok "Tornjak is ready."
    }

    Write-Ok "Metrics scenario is ready!"
    Write-Info "Tornjak UI:  http://localhost:3000"
    Write-Info "Tornjak API: http://localhost:10000"
    Write-Info "Prometheus:  http://localhost:9090"
    Write-Info "Graphite:    http://localhost:8080"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs "metrics-spire-server"
    Show-ContainerLogs "metrics-spire-agent"
    Show-ContainerLogs "metrics-tornjak-backend"
    Show-ContainerLogs "prometheus"
    Show-ContainerLogs "graphite"
    Show-PodmanStatus
    throw
}
finally {
    Pop-Location
}
