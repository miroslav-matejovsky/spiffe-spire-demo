$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$scenarioName = Split-Path -Leaf $scenarioRoot
$agentContainerName = "${scenarioName}_spire-agent_1"

Push-Location $scenarioRoot
try {
    Write-Host "Starting simple SPIRE scenario..." -ForegroundColor Cyan

    podman-compose up -d spire-server *>$null

    Write-Host "Waiting for SPIRE server..." -ForegroundColor Yellow
    $serverReady = $false
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $null = podman-compose exec -T spire-server /opt/spire/bin/spire-server healthcheck 2>&1
        if ($LASTEXITCODE -eq 0) {
            $serverReady = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $serverReady) {
        throw "SPIRE server did not become ready in time."
    }

    Write-Host "Generating join token..." -ForegroundColor Yellow
    $token = $null
    $tokenOutput = ""
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
        $tokenMatch = [regex]::Match($tokenOutput, 'Token:\s+(\S+)')
        if ($tokenMatch.Success) {
            $token = $tokenMatch.Groups[1].Value
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $token) {
        throw "Failed to generate a join token.`n$tokenOutput"
    }

    # Remove any leftover agent container from a previous run
    $existing = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existing -contains $agentContainerName) {
        podman rm -f $agentContainerName *>$null
    }

    Write-Host "Starting SPIRE agent with join token..." -ForegroundColor Yellow
    podman-compose run -d --name $agentContainerName spire-agent `
        -config /opt/spire/conf/agent/agent.conf `
        -joinToken $token *>$null

    Write-Host "Waiting for agent attestation..." -ForegroundColor Yellow
    $agentListOutput = ""
    $agentAttested = $false
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $agentListOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        if ($agentListOutput -match 'spiffe://mirmat\.org/spire/agent/join_token/' -and $agentListOutput -match 'Attestation type\s+: join_token') {
            $agentAttested = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $agentAttested) {
        throw "The SPIRE agent did not attest successfully.`n$agentListOutput"
    }

    Write-Host "`nRegistered agents:" -ForegroundColor Cyan
    Write-Host $agentListOutput.Trim()
    Write-Host "`nSimple scenario is ready!" -ForegroundColor Green
    Write-Host "  Server:  $($scenarioName)_spire-server_1" -ForegroundColor Gray
    Write-Host "  Agent:   $agentContainerName" -ForegroundColor Gray
} finally {
    Pop-Location
}
