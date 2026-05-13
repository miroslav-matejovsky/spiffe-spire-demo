# Scripts

## smoke-test.ps1

A scenario-aware smoke test for the SPIFFE/SPIRE demo. Tests basic functionality of each scenario's podman-compose stack.

### Usage

```powershell
# Run smoke test for a specific scenario
.\scripts\smoke-test.ps1 -Scenario 01-simple

# Run with verbose output
.\scripts\smoke-test.ps1 -Scenario 05-metrics -Verbose

# Run tests without starting containers (containers must already be running)
.\scripts\smoke-test.ps1 -Scenario 02-workload -SkipStartup
```

### Available Scenarios

| Scenario | Description |
|----------|-------------|
| `01-simple` | Basic SPIRE server and agent with join token attestation |
| `02-workload` | Workload identity with Unix attestor |
| `03-tpm` | TPM-based node attestation (certificate-based) |
| `04-svid-api` | mTLS between Go services using SVIDs |
| `05-metrics` | SPIRE telemetry with Prometheus and Graphite |

### What It Tests

For each scenario, the smoke test checks:

1. **Prerequisites** — Podman CLI availability
2. **Scenario Directory** — compose.yml present
3. **Running Scenario Detection** — Warns if containers from another scenario are running
4. **Compose Stack** — Starts containers using the scenario's `set-env.ps1` script
5. **Container Status** — Verifies all required containers are running
6. **Health Checks** — Scenario-specific endpoint checks (e.g., Prometheus, Graphite)
7. **Configuration Files** — Verifies SPIRE configs exist
8. **Port Mappings** — Checks expected port bindings (scenario-specific)

### Exit Codes

- `0`: All tests passed
- `1`: One or more tests failed, or prerequisites not met

### Example Output

```
🔍 Smoke test for scenario: Simple SPIRE Setup (01-simple)

========================================
Prerequisites
========================================
✅ PASS: Podman CLI available
   podman version 5.8.2
✅ PASS: Scenario directory
   compose.yml found
...
========================================
Test Summary
========================================
Scenario: Simple SPIRE Setup (01-simple)
Total Tests: 6
Passed: 6
Failed: 0

✅ All smoke tests passed!
```
