$ErrorActionPreference = "Stop"

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$RepoRoot = Resolve-Path (Join-Path $ScenarioRoot '..' '..')
Push-Location $ScenarioRoot

try {
    Write-Host "Starting SVID API scenario..." -ForegroundColor Cyan

    # Build Go services from the repo root so the Dockerfiles can use go.mod and go.sum
    Write-Host "Building Go services..." -ForegroundColor Yellow
    $serverDockerfile = Join-Path $RepoRoot 'scenarios' '04-svid-api' 'server' 'Dockerfile'
    $clientDockerfile = Join-Path $RepoRoot 'scenarios' '04-svid-api' 'client' 'Dockerfile'
    podman build -t spiffe-spire-demo-svid-server:local -f $serverDockerfile $RepoRoot
    podman build -t spiffe-spire-demo-svid-client:local -f $clientDockerfile $RepoRoot

    # Start SPIRE server first
    podman-compose up -d spire-server
    Write-Host "Waiting for SPIRE server..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5

    # Generate join token and start agent
    $tokenOutput = podman-compose exec -T spire-server /opt/spire/bin/spire-server token generate -spiffeID spiffe://mirmat.org/myagent 2>&1 | Out-String
    $token = [regex]::Match($tokenOutput, 'Token:\s+(\S+)').Groups[1].Value
    if (-not $token) {
        Write-Host "Failed to generate token. Output: $tokenOutput" -ForegroundColor Red
        exit 1
    }

    $env:SPIRE_AGENT_JOIN_TOKEN = $token
    podman-compose up -d spire-agent
    Start-Sleep -Seconds 5

    # Register workloads
    Write-Host "Registering workloads..." -ForegroundColor Yellow
    & "$PSScriptRoot\register-workloads.ps1"

    # Start the Go services
    Write-Host "Starting Go services..." -ForegroundColor Yellow
    podman-compose up -d --no-build svid-server
    Start-Sleep -Seconds 3
    podman-compose up -d --no-build svid-client

    Write-Host "`nSVID API scenario is ready!" -ForegroundColor Green
    Write-Host "  View server logs: podman-compose logs -f svid-server" -ForegroundColor Gray
    Write-Host "  View client logs: podman-compose logs -f svid-client" -ForegroundColor Gray
}
finally {
    Remove-Item Env:SPIRE_AGENT_JOIN_TOKEN -ErrorAction SilentlyContinue
    Pop-Location
}
