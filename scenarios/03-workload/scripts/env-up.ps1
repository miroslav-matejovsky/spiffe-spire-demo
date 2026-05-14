[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$scenarioRoot        = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$scenarioName        = Split-Path -Leaf $scenarioRoot
$serverContainerName = "workload-spire-server"
$agentContainerName  = "workload-spire-agent"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

$maxAttempts = 15

Push-Location $scenarioRoot
try {
    Write-Step "Starting workload scenario..."

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
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) {
        throw "Failed to generate join token. Output: $tokenOutput"
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
    $agentList = ""
    $agentReady = $false
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        Start-Sleep -Seconds 2
        $agentList = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        Write-Detail "attempt $attempt/${maxAttempts}: agent list output:`n$($agentList.Trim())"
        if ($agentList -match 'spiffe://mirmat\.org/spire/agent/join_token/[a-f0-9\-]+') {
            $agentReady = $true
            break
        }
    }

    if (-not $agentReady) {
        throw "SPIRE agent did not attest successfully after $maxAttempts attempts."
    }
    Write-Ok "Agent attested successfully."

    Write-Step "Starting workload container..."
    podman-compose up -d --no-deps workload *>$null
    Start-Sleep -Seconds 3

    Write-Step "Starting Tornjak..."
    podman-compose up -d tornjak-backend tornjak-frontend *>$null

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

    Write-Host "`n   Registered agents:" -ForegroundColor Cyan
    Write-Host $agentList.Trim()
    Write-Ok "Workload scenario is ready!"
    Write-Info "The workload container is watching the Workload API via the shared socket."
    Write-Info "Tornjak UI:  http://localhost:3000"
    Write-Info "Tornjak API: http://localhost:10000"
    Write-Info "Next: Run .\scripts\register-workload.ps1 to register a workload"
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs $serverContainerName
    Show-ContainerLogs $agentContainerName
    Show-ContainerLogs "workload-tornjak-backend"
    Show-PodmanStatus
    throw
}
finally {
    Pop-Location
}
