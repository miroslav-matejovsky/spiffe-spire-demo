[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$serverContainerName = "dashboard-spire-server"
$agentContainerName  = "dashboard-spire-agent"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

$maxAttempts = 15

Push-Location $scenarioRoot
try {
    Write-Step "Starting dashboard scenario..."

    Write-Step "Building dashboard..."
    podman build -t spiffe-spire-demo-dashboard:local -f (Join-Path $repoRoot "dashboard" "Containerfile") $repoRoot

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

    Write-Step "Generating join token..."
    $token = $null
    $tokenOutput = ""
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
        $tokenMatch = [regex]::Match($tokenOutput, 'Token:\s+(\S+)')
        Write-Detail "attempt $attempt/${maxAttempts}: token match=$($tokenMatch.Success)"
        if ($tokenMatch.Success) {
            $token = $tokenMatch.Groups[1].Value
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $token) {
        throw "Failed to generate a join token.`n$tokenOutput"
    }
    Write-Detail "join token acquired"

    $existing = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existing -contains $agentContainerName) {
        Write-Detail "removing leftover agent container: $agentContainerName"
        podman rm -f $agentContainerName *>$null
    }

    Write-Step "Starting SPIRE agent..."
    podman-compose run -d --name $agentContainerName spire-agent `
        -config /opt/spire/conf/agent/agent.conf `
        -joinToken $token *>$null

    Write-Step "Waiting for agent attestation..."
    $agentListOutput = ""
    $agentAttested = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $agentListOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        Write-Detail "attempt $attempt/${maxAttempts}: agent list output:`n$($agentListOutput.Trim())"
        if ($agentListOutput -match 'spiffe://mirmat\.org/spire/agent/join_token/' -and $agentListOutput -match 'Attestation type\s+: join_token') {
            $agentAttested = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $agentAttested) {
        throw "The SPIRE agent did not attest successfully after $maxAttempts attempts.`n$agentListOutput"
    }
    Write-Ok "Agent attested successfully."

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

    Write-Ok "Dashboard scenario is ready!"
    Write-Info "Dashboard: http://localhost:8080"
    Write-Info "Server:    $serverContainerName"
    Write-Info "Agent:     $agentContainerName"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs $serverContainerName
    Show-ContainerLogs $agentContainerName
    Show-ContainerLogs "dashboard-dashboard"
    Show-PodmanStatus
    throw
}
finally {
    Pop-Location
}
