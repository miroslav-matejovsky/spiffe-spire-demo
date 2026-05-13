$ErrorActionPreference = "Stop"

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    # Get agent SPIFFE ID
    $agentList = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
    $agentID = [regex]::Match($agentList, '(spiffe://example\.org/spire/agent/join_token/[a-f0-9\-]+)').Groups[1].Value
    if (-not $agentID) {
        Write-Host "No attested agent found. Ensure SPIRE agent is running." -ForegroundColor Red
        exit 1
    }

    Write-Host "Registering server workload..." -ForegroundColor Yellow
    podman-compose exec -T spire-server /opt/spire/bin/spire-server entry create `
        -spiffeID spiffe://example.org/svid-server `
        -parentID $agentID `
        -selector unix:uid:10001

    Write-Host "Registering client workload..." -ForegroundColor Yellow
    podman-compose exec -T spire-server /opt/spire/bin/spire-server entry create `
        -spiffeID spiffe://example.org/svid-client `
        -parentID $agentID `
        -selector unix:uid:10002

    Write-Host "Workloads registered!" -ForegroundColor Green
}
finally {
    Pop-Location
}
