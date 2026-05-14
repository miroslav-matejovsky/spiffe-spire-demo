[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

$scenarioRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

Push-Location $scenarioRoot
try {
    Write-Step "Discovering attested agents..."
    $agentList = podman-compose exec -T spire-server /opt/spire/bin/spire-server agent list 2>&1 | Out-String

    $agent1Match = [regex]::Match($agentList, '(spiffe://mirmat\.org/spire/agent/join_token/[a-f0-9\-]+)')
    if (-not $agent1Match.Success) {
        throw "Agent 1 (join_token) not found in agent list.`n$agentList"
    }
    $agent1ID = $agent1Match.Groups[1].Value
    Write-Detail "Agent 1 (join_token): $agent1ID"

    $agent2Match = [regex]::Match($agentList, '(spiffe://mirmat\.org/spire/agent/x509pop/[^\s]+)')
    if (-not $agent2Match.Success) {
        throw "Agent 2 (x509pop) not found in agent list.`n$agentList"
    }
    $agent2ID = $agent2Match.Groups[1].Value
    Write-Detail "Agent 2 (x509pop): $agent2ID"

    Write-Step "Registering workload-1 (join_token agent, unix:uid:0)..."
    podman-compose exec -T spire-server /opt/spire/bin/spire-server entry create `
        -spiffeID spiffe://mirmat.org/workload-1 `
        -parentID $agent1ID `
        -selector unix:uid:0 2>&1 | Write-Verbose

    Write-Step "Registering workload-2 (x509pop agent, unix:uid:0)..."
    podman-compose exec -T spire-server /opt/spire/bin/spire-server entry create `
        -spiffeID spiffe://mirmat.org/workload-2 `
        -parentID $agent2ID `
        -selector unix:uid:0 2>&1 | Write-Verbose

    Write-Ok "Workloads registered."
    Write-Info "workload-1: spiffe://mirmat.org/workload-1 (on join_token agent)"
    Write-Info "workload-2: spiffe://mirmat.org/workload-2 (on x509pop agent)"
}
finally {
    Pop-Location
}
