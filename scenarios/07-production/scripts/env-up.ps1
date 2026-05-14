[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$scenarioRoot     = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$repoRoot         = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
$agent2CertPath   = Join-Path $scenarioRoot "spire" "agent-2" "devid-cert.pem"
$agent2KeyPath    = Join-Path $scenarioRoot "spire" "agent-2" "devid-key.pem"
$serverCaPath     = Join-Path $scenarioRoot "spire" "server" "devid-ca.pem"
. (Join-Path $repoRoot "scripts" "logging.ps1")

$maxAttempts = 20

Push-Location $scenarioRoot
try {
    Write-Step "Starting production scenario..."

    if (-not (Test-Path $agent2CertPath) -or -not (Test-Path $agent2KeyPath) -or -not (Test-Path $serverCaPath)) {
        Write-Step "Provisioning x509pop credentials for agent-2..."
        & (Join-Path $PSScriptRoot "provision-agent.ps1")
    }

    Write-Step "Starting metrics collectors..."
    podman-compose up -d graphite-statsd prometheus *>$null

    Write-Step "Starting SPIRE server..."
    podman-compose up -d spire-server *>$null

    Write-Step "Waiting for SPIRE server to become ready..."
    $serverReady = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $null = podman-compose exec -T spire-server /opt/spire/bin/spire-server healthcheck 2>&1
        Write-Detail "attempt $attempt/${maxAttempts}: server healthcheck exit code $LASTEXITCODE"
        if ($LASTEXITCODE -eq 0) { $serverReady = $true; break }
        Start-Sleep -Seconds 2
    }
    if (-not $serverReady) { throw "SPIRE server did not become ready." }
    Write-Ok "SPIRE server is ready."

    Write-Step "Generating join token for agent-1..."
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) { throw "Failed to generate join token. Output: $tokenOutput" }
    Write-Detail "join token acquired"

    $existing = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existing -contains "production-spire-agent-1") {
        podman rm -f "production-spire-agent-1" *>$null
    }

    Write-Step "Starting agent-1 (join_token)..."
    $env:SPIRE_AGENT_1_JOIN_TOKEN = $token
    podman-compose up -d spire-agent-1 *>$null

    Write-Step "Starting agent-2 (x509pop)..."
    podman-compose up -d spire-agent-2 *>$null

    Write-Step "Waiting for both agents to attest..."
    $agentListOutput = ""
    $bothAttested = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $agentListOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        $hasAgent1 = $agentListOutput -match 'join_token'
        $hasAgent2 = $agentListOutput -match 'x509pop'
        Write-Detail "attempt $attempt/${maxAttempts}: agent1=$hasAgent1, agent2=$hasAgent2"
        if ($hasAgent1 -and $hasAgent2) { $bothAttested = $true; break }
        Start-Sleep -Seconds 3
    }
    if (-not $bothAttested) { throw "Not all agents attested. Output:`n$agentListOutput" }
    Write-Ok "Both agents attested successfully."

    Write-Step "Starting workload containers..."
    podman-compose up -d workload-1 workload-2 *>$null

    Write-Step "Registering workloads..."
    & (Join-Path $PSScriptRoot "register-workloads.ps1")

    Write-Step "Starting Tornjak..."
    podman-compose up -d tornjak-backend tornjak-frontend *>$null

    Write-Step "Waiting for Tornjak backend to become ready..."
    $tornjak_ready = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:10000/api/debugserver" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -lt 500) { $tornjak_ready = $true; break }
        } catch {}
        Write-Detail "attempt $attempt/${maxAttempts}: waiting for Tornjak..."
        Start-Sleep -Seconds 2
    }
    if (-not $tornjak_ready) {
        Write-Warn "Tornjak backend did not respond — it may still be starting."
    } else {
        Write-Ok "Tornjak is ready."
    }

    Write-Host "`n   Registered agents:" -ForegroundColor Cyan
    Write-Host $agentListOutput.Trim()

    Write-Ok "Production scenario is ready!"
    Write-Info "Tornjak UI:   http://localhost:3000"
    Write-Info "Tornjak API:  http://localhost:10000"
    Write-Info "Prometheus:   http://localhost:9090"
    Write-Info "Graphite:     http://localhost:8080"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs "production-spire-server"
    Show-ContainerLogs "production-spire-agent-1"
    Show-ContainerLogs "production-spire-agent-2"
    Show-ContainerLogs "production-tornjak-backend"
    Show-PodmanStatus
    throw
}
finally {
    Remove-Item Env:SPIRE_AGENT_1_JOIN_TOKEN -ErrorAction SilentlyContinue
    Pop-Location
}
