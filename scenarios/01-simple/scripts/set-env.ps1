$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot

Push-Location $scenarioRoot
try {
    Write-Host "Starting simple SPIRE scenario..." -ForegroundColor Cyan

    podman-compose up -d spire-server | Out-Null

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

    Write-Host "Starting SPIRE agent with join token..." -ForegroundColor Yellow
    $env:SPIRE_AGENT_JOIN_TOKEN = $token
    podman-compose up -d spire-agent | Out-Null

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
    Write-Host "  Server service: spire-server" -ForegroundColor Gray
    Write-Host "  Agent service:  spire-agent" -ForegroundColor Gray
} finally {
    Remove-Item Env:SPIRE_AGENT_JOIN_TOKEN -ErrorAction SilentlyContinue
    Pop-Location
}
