[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$scenarioRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$scenarioName = Split-Path -Leaf $scenarioRoot
$repoRoot     = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

$maxAttempts = 15

Push-Location $scenarioRoot
try {
    Write-Step "Starting SVID API scenario..."

    Write-Step "Building Go services..."
    $serverContainerfile = Join-Path $repoRoot 'scenarios' '05-svid-api' 'server' 'Containerfile'
    $clientContainerfile = Join-Path $repoRoot 'scenarios' '05-svid-api' 'client' 'Containerfile'
    podman build -t spiffe-spire-demo-dashboard:local -f (Join-Path $repoRoot "dashboard" "Containerfile") $repoRoot
    podman build -t spiffe-spire-demo-svid-server:local -f $serverContainerfile $repoRoot
    podman build -t spiffe-spire-demo-svid-client:local -f $clientContainerfile $repoRoot

    Write-Step "Starting SPIRE server..."
    podman-compose up -d spire-server *>$null

    Write-Step "Waiting for SPIRE server to become ready..."
    $serverReady = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $null = podman-compose exec -T spire-server /opt/spire/bin/spire-server healthcheck 2>&1
        Write-Detail "attempt $attempt/${maxAttempts}: healthcheck exit code $LASTEXITCODE"
        if ($LASTEXITCODE -eq 0) {
            $serverReady = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $serverReady) {
        throw "SPIRE server did not become ready after $maxAttempts attempts."
    }
    Write-Ok "SPIRE server is ready."

    Write-Step "Generating join token and starting SPIRE agent..."
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) {
        throw "Failed to generate token. Output: $tokenOutput"
    }
    Write-Detail "join token acquired"

    $env:SPIRE_AGENT_JOIN_TOKEN = $token
    podman-compose up -d spire-agent *>$null

    Write-Step "Waiting for agent attestation..."
    $agentListOutput = ""
    $agentAttested   = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $agentListOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        Write-Detail "attempt $attempt/${maxAttempts}: agent list output:`n$($agentListOutput.Trim())"
        if ($agentListOutput -match 'spiffe://mirmat\.org/spire/agent/join_token/') {
            $agentAttested = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $agentAttested) {
        throw "SPIRE agent did not attest successfully after $maxAttempts attempts."
    }
    Write-Ok "Agent attested successfully."

    Write-Step "Registering workloads..."
    & "$PSScriptRoot\register-workloads.ps1"

    Write-Step "Starting Go services..."
    podman-compose up -d --no-build svid-server *>$null
    Start-Sleep -Seconds 3
    podman-compose up -d --no-build svid-client *>$null

    Write-Step "Starting dashboard..."
    podman-compose up -d --no-build dashboard *>$null

    Write-Step "Waiting for dashboard to become ready..."
    $dashboardReady = $false
    for ($attempt = 1; $attempt -le 30; $attempt++) {
        try {
            $response = Invoke-WebRequest -Uri "http://127.0.0.1:8080/health" -UseBasicParsing -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response.StatusCode -eq 200) {
                $dashboardReady = $true
                break
            }
        } catch {}
        Write-Detail "attempt $attempt/30: waiting for dashboard..."
        Start-Sleep -Seconds 2
    }

    if (-not $dashboardReady) {
        Write-Warn "Dashboard did not respond in time -- it may still be starting."
    } else {
        Write-Ok "Dashboard is ready."
    }

    Write-Ok "SVID API scenario is ready!"
    Write-Info "Dashboard: http://localhost:8080"
    Write-Info "View server logs: podman-compose logs -f svid-server"
    Write-Info "View client logs: podman-compose logs -f svid-client"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs "svid-api-spire-server"
    Show-ContainerLogs "svid-api-spire-agent"
    Show-ContainerLogs "svid-api-svid-server"
    Show-ContainerLogs "svid-api-svid-client"
    Show-ContainerLogs "svid-api-dashboard"
    Show-PodmanStatus
    throw
}
finally {
    Remove-Item Env:SPIRE_AGENT_JOIN_TOKEN -ErrorAction SilentlyContinue
    Pop-Location
}
