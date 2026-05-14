[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$scenarioName = Split-Path -Leaf $scenarioRoot
$provisionScript = Join-Path $scenarioRoot "tpm\provision-tpm.ps1"
$serverCaPath    = Join-Path $scenarioRoot "spire\server\devid-ca.pem"
$agentCertPath   = Join-Path $scenarioRoot "spire\agent\devid-cert.pem"
$agentKeyPath    = Join-Path $scenarioRoot "spire\agent\devid-key.pem"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot ".." ".." "..")).Path
. (Join-Path $repoRoot "scripts" "logging.ps1")

$maxAttempts = 15

if (-not (Get-Command podman-compose -ErrorAction SilentlyContinue)) {
    throw "podman-compose was not found on PATH. Install it and try again."
}

Push-Location $scenarioRoot
try {
    Write-Step "Starting TPM learning scenario..."

    if (-not (Test-Path $serverCaPath) -or -not (Test-Path $agentCertPath) -or -not (Test-Path $agentKeyPath)) {
        Write-Warn "DevID materials not found. Running provisioning first..."
        & $provisionScript
    }

    Write-Step "Starting all containers..."
    podman-compose up -d *>$null
    if ($LASTEXITCODE -ne 0) {
        throw "podman-compose up failed."
    }

    Write-Step "Waiting for SPIRE services to initialize..."
    $containers = $null
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        Start-Sleep -Seconds 2
        $containers = @(podman ps --format "{{.Names}}")
        $serverRunning = $containers | Where-Object { $_ -match "spire-server" }
        $agentRunning  = $containers | Where-Object { $_ -match "spire-agent" }
        Write-Detail "attempt $attempt/${maxAttempts}: server=$([bool]$serverRunning) agent=$([bool]$agentRunning)"
        if ($serverRunning -and $agentRunning) { break }
    }

    foreach ($name in @("spire-server", "spire-agent")) {
        if ($containers -match $name) {
            Write-Ok "$name is running"
        }
        else {
            Write-Host "   ❌ $name is NOT running" -ForegroundColor Red
        }
    }

    $serverContainer = $containers | Where-Object { $_ -match "spire-server" } | Select-Object -First 1
    if (-not $serverContainer) {
        throw "Could not find the SPIRE Server container."
    }

    Write-Step "Waiting for agent attestation..."
    $agentList = $null
    $attested   = $false

    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        $agentList = podman exec $serverContainer /opt/spire/bin/spire-server agent list 2>&1
        Write-Detail "attempt $attempt/${maxAttempts}: exit code $LASTEXITCODE`n$(($agentList | Out-String).Trim())"
        if ($LASTEXITCODE -eq 0 -and ($agentList -match "spiffe://mirmat.org" -or $agentList -match "x509pop")) {
            $attested = $true
            break
        }
        Start-Sleep -Seconds 2
    }

    if ($attested) {
        Write-Ok "Agent attestation detected."
        Write-Host "`n$(($agentList | Out-String).TrimEnd())"
    }
    else {
        Write-Warn "The stack started, but attestation was not confirmed after $maxAttempts attempts."
        Write-Info "Check logs with: podman-compose logs -f -t"
    }

    Write-Ok "TPM learning scenario is ready."
}
catch {
    Write-Host "`n   ❌ Startup failed: $_" -ForegroundColor Red
    Show-ContainerLogs "${scenarioName}_spire-server_1"
    Show-ContainerLogs "${scenarioName}_spire-agent_1"
    Show-PodmanStatus
    throw
}
finally {
    Pop-Location
}
