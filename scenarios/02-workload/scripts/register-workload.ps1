$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Registering workload..." -ForegroundColor Cyan

    # Get the agent SPIFFE ID
    $agentList = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String
    $agentID = [regex]::Match($agentList, '(spiffe://mirmat\.org/spire/agent/join_token/[a-f0-9\-]+)').Groups[1].Value
    if (-not $agentID) {
        Write-Host "No attested agent found. Run set-env.ps1 first." -ForegroundColor Red
        exit 1
    }
    Write-Host "  Agent SPIFFE ID: $agentID" -ForegroundColor Gray

    # Create workload registration entry
    podman-compose exec -T spire-server /opt/spire/bin/spire-server entry create `
        -spiffeID spiffe://mirmat.org/myworkload `
        -parentID $agentID `
        -selector unix:uid:0

    Write-Host "`nWorkload registered successfully!" -ForegroundColor Green
    Write-Host "  SPIFFE ID: spiffe://mirmat.org/myworkload" -ForegroundColor Gray
    Write-Host "  Selector:  unix:uid:0 (root user)" -ForegroundColor Gray
    Write-Host "`nNext: Run .\scripts\fetch-svid.ps1 to display the workload SVID" -ForegroundColor Gray
}
finally {
    Pop-Location
}
