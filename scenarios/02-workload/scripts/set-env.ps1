$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Starting workload scenario..." -ForegroundColor Cyan

    # Start SPIRE server first
    podman-compose up -d spire-server | Out-Null
    Write-Host "Waiting for SPIRE server to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5

    # Generate join token
    Write-Host "Generating join token..." -ForegroundColor Yellow
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) {
        Write-Host "Failed to generate join token. Output: $tokenOutput" -ForegroundColor Red
        exit 1
    }
    Write-Host "  Token: $token" -ForegroundColor Gray

    # Start agent with join token
    Write-Host "Starting SPIRE agent..." -ForegroundColor Yellow
    $env:SPIRE_AGENT_JOIN_TOKEN = $token
    podman-compose up -d spire-agent | Out-Null

    # Wait for agent attestation before starting the workload container
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
        Write-Host "SPIRE agent did not attest successfully. Output: $agentList" -ForegroundColor Red
        exit 1
    }

    # Start workload container without recreating the agent service
    Write-Host "Starting workload container..." -ForegroundColor Yellow
    podman-compose up -d --no-deps workload | Out-Null
    Start-Sleep -Seconds 3

    # Verify
    Write-Host "Verifying agent attestation..." -ForegroundColor Yellow
    Write-Host $agentList.Trim()

    Write-Host "`nWorkload scenario is ready!" -ForegroundColor Green
    Write-Host "  The workload container is watching the Workload API via the shared socket." -ForegroundColor Gray
    Write-Host "  Next: Run .\scripts\register-workload.ps1 to register a workload" -ForegroundColor Gray
}
finally {
    Remove-Item Env:SPIRE_AGENT_JOIN_TOKEN -ErrorAction SilentlyContinue
    Pop-Location
}
