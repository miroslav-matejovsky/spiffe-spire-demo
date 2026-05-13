$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$projectName = Split-Path -Leaf $scenarioRoot
$serverContainerName = "${projectName}_spire-server_1"
$agentContainerName = "spire-simple-agent"

Push-Location $scenarioRoot
try {
    Write-Host "Starting simple SPIRE scenario..." -ForegroundColor Cyan

    podman-compose up -d spire-server | Out-Null

    Write-Host "Waiting for SPIRE server..." -ForegroundColor Yellow
    $serverReady = $false
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $runningContainers = @(podman ps --format "{{.Names}}" 2>$null)
        if ($runningContainers -contains $serverContainerName) {
            $serverReady = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if (-not $serverReady) {
        throw "Unable to find the SPIRE server container '$serverContainerName'."
    }

    Write-Host "Generating join token..." -ForegroundColor Yellow
    $token = $null
    $tokenOutput = ""
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $tokenOutput = podman exec $serverContainerName /opt/spire/bin/spire-server token generate -spiffeID spiffe://example.org/myagent 2>&1 | Out-String
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

    $existingContainers = @(podman ps -a --format "{{.Names}}" 2>$null)
    if ($existingContainers -contains $agentContainerName) {
        podman rm -f $agentContainerName | Out-Null
    }

    Write-Host "Starting SPIRE agent with join token..." -ForegroundColor Yellow
    podman-compose run -d --name $agentContainerName spire-agent -config /opt/spire/conf/agent/agent.conf -joinToken $token | Out-Null

    Write-Host "Waiting for agent attestation..." -ForegroundColor Yellow
    $agentListOutput = ""
    $agentAttested = $false
    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $agentListOutput = podman exec $serverContainerName /opt/spire/bin/spire-server agent list 2>&1 | Out-String
        if ($agentListOutput -match 'spiffe://example\.org/spire/agent/join_token/' -and $agentListOutput -match 'Attestation type\s+: join_token') {
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
    Write-Host "  Server service: spire-server" -ForegroundColor Gray
    Write-Host "  Agent container: $agentContainerName" -ForegroundColor Gray
} finally {
    Pop-Location
}
