$ErrorActionPreference = "Stop"

$scenarioRoot = Split-Path -Parent $PSScriptRoot
$provisionScript = Join-Path $scenarioRoot "tpm\provision-tpm.ps1"
$serverCaPath = Join-Path $scenarioRoot "spire\server\devid-ca.pem"
$agentCertPath = Join-Path $scenarioRoot "spire\agent\devid-cert.pem"
$agentKeyPath = Join-Path $scenarioRoot "spire\agent\devid-key.pem"

if (-not (Get-Command podman-compose -ErrorAction SilentlyContinue)) {
    throw "podman-compose was not found on PATH. Install it and try again."
}

Push-Location $scenarioRoot
try {
    Write-Host "Starting TPM learning scenario..." -ForegroundColor Cyan

    if (-not (Test-Path $serverCaPath) -or -not (Test-Path $agentCertPath) -or -not (Test-Path $agentKeyPath)) {
        Write-Host "DevID materials not found. Running provisioning first..." -ForegroundColor Yellow
        & $provisionScript
    }

    podman-compose up -d *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "podman-compose up failed."
    }

    Write-Host "Waiting for SPIRE services to initialize..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10

    $containers = podman ps --format "{{.Names}}"
    foreach ($name in @("spire-server", "spire-agent")) {
        if ($containers -match $name) {
            Write-Host "  ✅ $name is running" -ForegroundColor Green
        }
        else {
            Write-Host "  ❌ $name is NOT running" -ForegroundColor Red
        }
    }

    $serverContainer = $containers | Where-Object { $_ -match "spire-server" } | Select-Object -First 1
    if (-not $serverContainer) {
        throw "Could not find the SPIRE Server container."
    }

    $agentList = $null
    $attested = $false

    for ($attempt = 1; $attempt -le 15; $attempt++) {
        $agentList = podman exec $serverContainer /opt/spire/bin/spire-server agent list 2>&1
        if ($LASTEXITCODE -eq 0 -and ($agentList -match "spiffe://mirmat.org" -or $agentList -match "x509pop")) {
            $attested = $true
            break
        }

        Start-Sleep -Seconds 2
    }

    if ($attested) {
        Write-Host "`nAgent attestation detected:" -ForegroundColor Green
        Write-Host (($agentList | Out-String).TrimEnd())
    }
    else {
        Write-Host "`nThe stack started, but attestation was not confirmed yet." -ForegroundColor Yellow
        Write-Host "Check logs with: podman-compose logs -f -t" -ForegroundColor Gray
    }

    Write-Host "`nTPM learning scenario is ready." -ForegroundColor Green
}
finally {
    Pop-Location
}
