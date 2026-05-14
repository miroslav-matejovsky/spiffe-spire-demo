# Smoke test for SPIFFE/SPIRE demo scenarios
# Tests basic functionality of a given scenario's podman-compose stack

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("01-simple", "02-dashboard", "03-workload", "04-tpm", "05-svid-api", "06-metrics", "07-production")]
    [string]$Scenario,
    [switch]$SkipStartup = $false
)

$ErrorActionPreference = "Stop"
$VerbosePreference = if ($PSBoundParameters.ContainsKey("Verbose")) { "Continue" } else { "SilentlyContinue" }

$TestsPassed = 0
$TestsFailed = 0

function Write-TestHeader {
    param([string]$Title)
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host $Title -ForegroundColor Cyan
    Write-Host "========================================`n" -ForegroundColor Cyan
}

function Test-Pass {
    param([string]$TestName, [string]$Message = "")
    $script:TestsPassed++
    Write-Host "✅ PASS: $TestName" -ForegroundColor Green
    if ($Message) { Write-Host "   $Message" -ForegroundColor Gray }
}

function Test-Fail {
    param([string]$TestName, [string]$Message = "")
    $script:TestsFailed++
    Write-Host "❌ FAIL: $TestName" -ForegroundColor Red
    if ($Message) { Write-Host "   $Message" -ForegroundColor Gray }
}

$ScenarioConfig = @{
    "01-simple" = @{
        Name = "Simple SPIRE Setup"
        ContainerPatterns = @("spire-server", "spire-agent")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf")
        PortMappings = @{}
        HealthChecks = @{}
    }
    "02-dashboard" = @{
        Name = "SPIRE Dashboard"
        ContainerPatterns = @("spire-server", "spire-agent", "dashboard")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf")
        PortMappings = @{
            "Dashboard" = "8080"
        }
        HealthChecks = @{}
    }
    "03-workload" = @{
        Name = "Workload Identity"
        ContainerPatterns = @("spire-server", "spire-agent", "workload", "dashboard")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf")
        PortMappings = @{
            "Dashboard" = "8080"
        }
        HealthChecks = @{}
    }
    "04-tpm" = @{
        Name = "TPM-Based Attestation"
        ContainerPatterns = @("spire-server", "spire-agent", "dashboard")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf")
        PortMappings = @{
            "Dashboard" = "8080"
        }
        HealthChecks = @{}
    }
    "05-svid-api" = @{
        Name = "SVID-Based Service API"
        ContainerPatterns = @("spire-server", "spire-agent", "svid-server", "svid-client", "dashboard")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf")
        PortMappings = @{
            "Dashboard" = "8080"
        }
        HealthChecks = @{}
    }
    "06-metrics" = @{
        Name = "SPIRE Telemetry & Metrics"
        ContainerPatterns = @("spire-server", "spire-agent", "prometheus", "graphite", "dashboard")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent/agent.conf", "./prometheus/prometheus.yml")
        PortMappings = @{
            "Dashboard" = "8090"
            "Graphite Web UI" = "8080"
            "Graphite StatsD" = "8125"
            "Prometheus Web UI" = "9090"
        }
        HealthChecks = @{
            "Prometheus" = @{ Container = "prometheus"; URL = "http://localhost:9090/-/healthy"; Match = "Healthy" }
            "Graphite" = @{ Container = "graphite"; URL = "http://localhost:80/"; Match = "Graphite|<html" }
        }
    }
    "07-production" = @{
        Name = "Production-Like Setup"
        ContainerPatterns = @("spire-server", "spire-agent-1", "spire-agent-2", "workload-1", "workload-2", "dashboard", "prometheus", "graphite")
        ConfigFiles = @("./spire/server/server.conf", "./spire/agent-1/agent.conf", "./spire/agent-2/agent.conf", "./prometheus/prometheus.yml")
        PortMappings = @{
            "Dashboard" = "8090"
            "Prometheus" = "9090"
            "Graphite" = "8080"
        }
        HealthChecks = @{
            "Prometheus" = @{ Container = "production-prometheus"; URL = "http://localhost:9090/-/healthy"; Match = "Healthy" }
            "Graphite" = @{ Container = "production-graphite"; URL = "http://localhost:80/"; Match = "Graphite|<html" }
        }
    }
}

$config = $ScenarioConfig[$Scenario]
$scenarioDir = Join-Path $PSScriptRoot '..' 'scenarios' $Scenario

Write-Host "🔍 Smoke test for scenario: $($config.Name) ($Scenario)" -ForegroundColor Magenta

Write-TestHeader "Prerequisites"
try {
    $podmanVersion = podman --version 2>&1
    Test-Pass "Podman CLI available" $podmanVersion
} catch {
    Test-Fail "Podman CLI available" "Error: $($_.Exception.Message)"
    exit 1
}

if (-not (Test-Path (Join-Path $scenarioDir "compose.yml"))) {
    Test-Fail "Scenario directory" "compose.yml not found in $scenarioDir"
    exit 1
}
Test-Pass "Scenario directory" "compose.yml found in $scenarioDir"

Write-TestHeader "Running Scenario Detection"
$runningContainers = podman ps --format "{{.Names}}" 2>&1 | Out-String
$otherScenariosRunning = @()
foreach ($otherScenario in $ScenarioConfig.Keys) {
    if ($otherScenario -eq $Scenario) { continue }
    foreach ($pattern in $ScenarioConfig[$otherScenario].ContainerPatterns) {
        if ($runningContainers -match $pattern) {
            $otherScenariosRunning += $otherScenario
            break
        }
    }
}

if ($otherScenariosRunning.Count -gt 0) {
    $uniqueScenarios = $otherScenariosRunning | Sort-Object -Unique
    Write-Host "⚠️  WARNING: Containers from other scenario(s) detected: $($uniqueScenarios -join ', ')" -ForegroundColor Yellow
    Write-Host "   Please teardown running scenarios before starting a new one:" -ForegroundColor Yellow
    foreach ($s in $uniqueScenarios) {
        Write-Host "     cd scenarios\$s && podman-compose down" -ForegroundColor Yellow
    }
    Write-Host ""
}
Test-Pass "Scenario detection" "Check complete"

Write-TestHeader "Compose Stack"
Push-Location $scenarioDir
try {
    if (-not $SkipStartup) {
        Write-Verbose "Starting compose stack..."
        try {
            $setEnvScript = Join-Path $scenarioDir 'scripts' 'env-up.ps1'
            if (Test-Path $setEnvScript) {
                & $setEnvScript
            } else {
                podman-compose up -d 2>&1 | ForEach-Object { Write-Verbose $_ }
            }
            Start-Sleep -Seconds 5
            Test-Pass "Compose stack startup"
        } catch {
            Test-Fail "Compose stack startup" "Error: $($_.Exception.Message)"
            exit 1
        }
    } else {
        Write-Verbose "Skipping startup (-SkipStartup flag set)"
        Test-Pass "Compose stack startup" "Skipped (-SkipStartup)"
    }

    $runningContainers = podman ps --format "{{.Names}}" 2>&1
    $allRunning = $true
    foreach ($pattern in $config.ContainerPatterns) {
        if ($runningContainers -match $pattern) {
            Test-Pass "Container running: $pattern"
        } else {
            Test-Fail "Container running: $pattern"
            $allRunning = $false
        }
    }

    if (-not $allRunning) {
        Write-Host "`nRunning containers:" -ForegroundColor Yellow
        podman ps --format "table {{.Names}}`t{{.Status}}"
    }

    if ($config.HealthChecks.Count -gt 0) {
        Write-TestHeader "Service Health Checks"
        foreach ($check in $config.HealthChecks.GetEnumerator()) {
            try {
                $response = podman exec $check.Value.Container wget -q -O- $check.Value.URL 2>&1
                if ($response -match $check.Value.Match) {
                    Test-Pass "$($check.Key) health check" "Accessible"
                } else {
                    Test-Fail "$($check.Key) health check" "Unexpected response"
                }
            } catch {
                Test-Fail "$($check.Key) health check" "Error: $($_.Exception.Message)"
            }
        }
    }

    Write-TestHeader "Configuration Files"
    foreach ($file in $config.ConfigFiles) {
        if (Test-Path $file) {
            Test-Pass "Config file exists: $file"
        } else {
            Test-Fail "Config file exists: $file"
        }
    }

    if ($config.PortMappings.Count -gt 0) {
        Write-TestHeader "Port Mappings"
        foreach ($service in $config.PortMappings.GetEnumerator()) {
            $port = $service.Value
            # Match host port binding specifically (e.g. "0.0.0.0:8090->") to avoid false positives
            $containers = podman ps --format "{{.Ports}}" 2>&1 | Select-String ":${port}->"
            if ($containers) {
                Test-Pass "Port $port mapped ($($service.Key))"
            } else {
                Test-Fail "Port $port mapped ($($service.Key))" "Run 'podman ps' for details"
            }
        }
    }
} finally {
    Pop-Location
}

Write-TestHeader "Test Summary"
$totalTests = $TestsPassed + $TestsFailed
Write-Host "Scenario: $($config.Name) ($Scenario)" -ForegroundColor Cyan
Write-Host "Total Tests: $totalTests" -ForegroundColor Cyan
Write-Host "Passed: $TestsPassed" -ForegroundColor Green
Write-Host "Failed: $TestsFailed" -ForegroundColor $(if ($TestsFailed -gt 0) { "Red" } else { "Green" })

if ($TestsFailed -eq 0) {
    Write-Host "`n✅ All smoke tests passed!" -ForegroundColor Green
    exit 0
} else {
    Write-Host "`n❌ Some tests failed. Review the output above." -ForegroundColor Red
    exit 1
}
