$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Reading SVID fetched by the workload container..." -ForegroundColor Cyan

    $workloadContainer = "workload-workload"
    if (-not (podman ps --filter "name=^workload-workload$" --format "{{.Names}}" 2>$null)) {
        Write-Host "Workload container is not running. Run env-up.ps1 first." -ForegroundColor Red
        exit 1
    }

    $logs = ""
    for ($attempt = 1; $attempt -le 60; $attempt++) {
        $logs = podman logs $workloadContainer 2>&1 | Out-String
        if ($logs -match 'Received 1 svid') {
            break
        }
        Start-Sleep -Seconds 2
    }

    if ($logs -notmatch 'Received 1 svid') {
        Write-Host "The workload has not received an SVID yet. Make sure register-workload.ps1 completed successfully." -ForegroundColor Red
        exit 1
    }

    $logs -split "`r?`n" |
        Where-Object { $_ -match '^Received 1 svid' -or $_ -match '^SPIFFE ID:' -or $_ -match '^SVID Valid After:' -or $_ -match '^SVID Valid Until:' -or $_ -match '^CA #' } |
        ForEach-Object { Write-Host $_ }

    Write-Host "`nSVID fetched successfully!" -ForegroundColor Green
    Write-Host "The output above shows the X.509-SVID issued to the workload." -ForegroundColor Gray
}
finally {
    Pop-Location
}
