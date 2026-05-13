$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$ScenarioRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $ScenarioRoot

try {
    Write-Host "Stopping workload scenario..." -ForegroundColor Cyan

    podman-compose down -v --remove-orphans | Out-Null

    foreach ($containerName in @("workload", "spire-agent", "02-workload_spire-agent_1")) {
        $container = podman ps -a --format "{{.Names}}" | Where-Object { $_ -eq $containerName }
        if ($container) {
            podman rm -f $containerName | Out-Null
        }
    }

    Write-Host "Workload scenario stopped." -ForegroundColor Green
}
finally {
    Pop-Location
}
