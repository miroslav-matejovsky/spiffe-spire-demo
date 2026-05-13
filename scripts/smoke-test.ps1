# Smoke test for SPIFFE/SPIRE demo with Prometheus and Graphite
# Tests basic functionality of the podman-compose stack

param(
    [switch]$SkipStartup = $false,
    [switch]$Verbose = $false
)

$ErrorActionPreference = "Stop"
$VerbosePreference = if ($Verbose) { "Continue" } else { "SilentlyContinue" }

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
    $TestsPassed++
    Write-Host "✅ PASS: $TestName" -ForegroundColor Green
    if ($Message) { Write-Host "   $Message" -ForegroundColor Gray }
}

function Test-Fail {
    param([string]$TestName, [string]$Message = "")
    $TestsFailed++
    Write-Host "❌ FAIL: $TestName" -ForegroundColor Red
    if ($Message) { Write-Host "   $Message" -ForegroundColor Gray }
}

# Test 1: Check if podman is available
Write-TestHeader "Prerequisites"
try {
    $podmanVersion = podman --version 2>&1
    Test-Pass "Podman CLI available" $podmanVersion
} catch {
    Test-Fail "Podman CLI available" "Error: $($_.Exception.Message)"
    exit 1
}

# Test 2: Verify we're in the correct directory
if (-not (Test-Path "compose.yml")) {
    Test-Fail "Working directory" "compose.yml not found in current directory"
    exit 1
}
Test-Pass "Working directory" "compose.yml found"

# Test 3: Start compose stack if requested
Write-TestHeader "Docker Compose Stack"
if (-not $SkipStartup) {
    Write-Verbose "Starting compose stack..."
    try {
        podman-compose up -d 2>&1 | ForEach-Object { Write-Verbose $_ }
        Start-Sleep -Seconds 5
        Test-Pass "Compose stack startup"
    } catch {
        Test-Fail "Compose stack startup" "Error: $($_.Exception.Message)"
        exit 1
    }
} else {
    Write-Verbose "Skipping startup (-SkipStartup flag set)"
}

# Test 4: Check if all required containers are running
$requiredContainers = @("prometheus", "graphite", "spiffe-spire-demo_spire-server_1", "spiffe-spire-demo_spire-agent_1")
$runningContainers = podman ps --format "{{.Names}}" 2>&1

$allRunning = $true
foreach ($container in $requiredContainers) {
    if ($runningContainers -match $container) {
        Test-Pass "Container running: $container"
    } else {
        Test-Fail "Container running: $container"
        $allRunning = $false
    }
}

if (-not $allRunning) {
    Write-Host "`nRunning containers:" -ForegroundColor Yellow
    podman ps --format "table {{.Names}}\t{{.Status}}"
}

# Test 5: Test service endpoints from inside containers
Write-TestHeader "Service Health Checks"

# Prometheus health check
try {
    $prometheusHealth = podman exec prometheus wget -q -O- "http://localhost:9090/-/healthy" 2>&1
    if ($prometheusHealth -match "Healthy") {
        Test-Pass "Prometheus health endpoint" "Response: $prometheusHealth"
    } else {
        Test-Fail "Prometheus health endpoint" "Unexpected response: $prometheusHealth"
    }
} catch {
    Test-Fail "Prometheus health endpoint" "Error: $($_.Exception.Message)"
}

# Prometheus targets check
try {
    $targets = podman exec prometheus wget -q -O- "http://localhost:9090/api/v1/targets" 2>&1
    if ($targets -match '"health":"up"' -or $targets -match '"health"' ) {
        Test-Pass "Prometheus targets API" "Accessible"
    } else {
        Test-Fail "Prometheus targets API" "Response: $targets"
    }
} catch {
    Test-Fail "Prometheus targets API" "Error: $($_.Exception.Message)"
}

# Graphite health check
try {
    $graphiteResponse = podman exec graphite wget -q -O- "http://localhost:80/" 2>&1
    if ($graphiteResponse -match "Graphite" -or $graphiteResponse -match "<html") {
        Test-Pass "Graphite web UI" "Accessible"
    } else {
        Test-Fail "Graphite web UI" "Unexpected response"
    }
} catch {
    Test-Fail "Graphite web UI" "Error: $($_.Exception.Message)"
}

# Test 6: Verify configuration files
Write-TestHeader "Configuration Files"

$configFiles = @(
    "./prometheus/prometheus.yml",
    "./spire/server/server.conf",
    "./spire/agent/agent.conf"
)

foreach ($file in $configFiles) {
    if (Test-Path $file) {
        Test-Pass "Config file exists: $file"
    } else {
        Test-Fail "Config file exists: $file"
    }
}

# Test 7: Verify Prometheus configuration
Write-TestHeader "Prometheus Configuration"
try {
    $promConfig = Get-Content "./prometheus/prometheus.yml" -Raw
    $scrapeConfigs = @("spire-server", "spire-agent")
    
    foreach ($config in $scrapeConfigs) {
        if ($promConfig -match $config) {
            Test-Pass "Prometheus scrape config: $config"
        } else {
            Test-Fail "Prometheus scrape config: $config"
        }
    }
} catch {
    Test-Fail "Reading Prometheus config" "Error: $($_.Exception.Message)"
}

# Test 8: Verify SPIRE configuration for telemetry
Write-TestHeader "SPIRE Telemetry Configuration"
try {
    $serverConfig = Get-Content "./spire/server/server.conf" -Raw
    if ($serverConfig -match "Prometheus" -and $serverConfig -match "Statsd") {
        Test-Pass "SPIRE server telemetry configured"
    } else {
        Test-Fail "SPIRE server telemetry configured"
    }
    
    $agentConfig = Get-Content "./spire/agent/agent.conf" -Raw
    if ($agentConfig -match "Prometheus" -and $agentConfig -match "Statsd") {
        Test-Pass "SPIRE agent telemetry configured"
    } else {
        Test-Fail "SPIRE agent telemetry configured"
    }
} catch {
    Test-Fail "Reading SPIRE configs" "Error: $($_.Exception.Message)"
}

# Test 9: Check port mappings
Write-TestHeader "Port Mappings"
$portChecks = @{
    "Graphite Web UI" = "8080"
    "Graphite StatsD" = "8125"
    "Prometheus Web UI" = "9090"
}

foreach ($service in $portChecks.Keys) {
    $port = $portChecks[$service]
    $containers = podman ps --format "{{.Ports}}" 2>&1 | Select-String $port
    if ($containers) {
        Test-Pass "Port $port mapped ($service)"
    } else {
        Test-Fail "Port $port mapped ($service)" "Run 'podman ps' for details"
    }
}

# Summary
Write-TestHeader "Test Summary"
$totalTests = $TestsPassed + $TestsFailed
Write-Host "Total Tests: $totalTests" -ForegroundColor Cyan
Write-Host "Passed: $TestsPassed" -ForegroundColor Green
Write-Host "Failed: $TestsFailed" -ForegroundColor $(if ($TestsFailed -gt 0) { "Red" } else { "Green" })

if ($TestsFailed -eq 0) {
    Write-Host "`n✅ All smoke tests passed!" -ForegroundColor Green
    Write-Host "`n📍 Access URLs:" -ForegroundColor Cyan
    Write-Host "   - Prometheus: http://localhost:9090" -ForegroundColor Gray
    Write-Host "   - Graphite:   http://localhost:8080" -ForegroundColor Gray
    exit 0
} else {
    Write-Host "`n❌ Some tests failed. Review the output above." -ForegroundColor Red
    exit 1
}
