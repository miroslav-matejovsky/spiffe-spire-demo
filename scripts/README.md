# Scripts

## smoke-test.ps1

A comprehensive smoke test for the SPIFFE/SPIRE demo with Prometheus and Graphite monitoring.

### Usage

```powershell
# Run full test (starts containers, runs all tests)
.\scripts\smoke-test.ps1

# Run tests with verbose output
.\scripts\smoke-test.ps1 -Verbose

# Run tests without starting containers (containers must already be running)
.\scripts\smoke-test.ps1 -SkipStartup
```

### What It Tests

1. **Prerequisites**
   - Podman CLI availability
   - Working directory validation (compose.yml present)

2. **Docker Compose Stack**
   - Compose stack startup
   - Container availability (Prometheus, Graphite, SPIRE server, SPIRE agent)

3. **Service Health Checks**
   - Prometheus health endpoint (`/-/healthy`)
   - Prometheus targets API (`/api/v1/targets`)
   - Graphite web UI accessibility

4. **Configuration Files**
   - Prometheus configuration
   - SPIRE server configuration
   - SPIRE agent configuration

5. **Prometheus Configuration**
   - SPIRE server scrape config
   - SPIRE agent scrape config

6. **SPIRE Telemetry Configuration**
   - Server telemetry (Prometheus + StatsD)
   - Agent telemetry (Prometheus + StatsD)

7. **Port Mappings**
   - Graphite Web UI (8080)
   - Graphite StatsD (8125)
   - Prometheus Web UI (9090)

### Exit Codes

- `0`: All tests passed
- `1`: One or more tests failed, or prerequisites not met

### Access URLs (after successful test)

- **Prometheus**: http://localhost:9090
- **Graphite**: http://localhost:8080

### Example Output

```
========================================
Prerequisites
========================================
✅ PASS: Podman CLI available
   podman version 5.8.2
✅ PASS: Working directory
   compose.yml found
...
========================================
Test Summary
========================================
Total Tests: 14
Passed: 14
Failed: 0

✅ All smoke tests passed!

📍 Access URLs:
   - Prometheus: http://localhost:9090
   - Graphite:   http://localhost:8080
```
