$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$scenarioName = Split-Path -Leaf $ScenarioRoot
$agentContainerName = "${scenarioName}_spire-agent_1"

Push-Location $ScenarioRoot
try {
    Write-Host "Starting workload scenario..." -ForegroundColor Cyan

    podman-compose up -d spire-server *>$null
    Write-Host "Waiting for SPIRE server to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5

    Write-Host "Generating join token..." -ForegroundColor Yellow
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) {
        throw "Failed to generate join token. Output: $tokenOutput"
    }
    Write-Host "  Token: $token" -ForegroundColor Gray

    # Remove any leftover agent container from a previous run
    $existing = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existing -contains $agentContainerName) {
        podman rm -f $agentContainerName *>$null
    }

    Write-Host "Starting SPIRE agent..." -ForegroundColor Yellow
    podman-compose run -d --name $agentContainerName spire-agent `
        -config /opt/spire/conf/agent/agent.conf `
        -joinToken $token *>$null

    Write-Host "Waiting for agent attestation..." -ForegroundColor Yellow
    $agentList = ""
    $agentReady = $false
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        Start-Sleep -Seconds 2
        $agentList = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        if ($agentList -match 'spiffe://mirmat\.org/spire/agent/join_token/[a-f0-9\-]+') {
            $agentReady = $true
            break
        }
    }
    if (-not $agentReady) {
        throw "SPIRE agent did not attest successfully. Output: $agentList"
    }

    Write-Host "Starting workload container..." -ForegroundColor Yellow
    podman-compose up -d --no-deps workload *>$null
    Start-Sleep -Seconds 3

    Write-Host $agentList.Trim()
    Write-Host "`nWorkload scenario is ready!" -ForegroundColor Green
    Write-Host "  The workload container is watching the Workload API via the shared socket." -ForegroundColor Gray
    Write-Host "  Next: Run .\scripts\register-workload.ps1 to register a workload" -ForegroundColor Gray
}
finally {
    Pop-Location
}
