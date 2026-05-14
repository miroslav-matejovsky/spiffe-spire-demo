# Scripts

## smoke-test.ps1

A scenario-aware smoke test for the SPIFFE/SPIRE demo. Tests basic functionality of each scenario's podman-compose stack.

### Usage

```powershell
# Run smoke test for a specific scenario
.\scripts\smoke-test.ps1 -Scenario 01-simple

# Run with verbose output
.\scripts\smoke-test.ps1 -Scenario 06-metrics -Verbose

# Run tests without starting containers (containers must already be running)
.\scripts\smoke-test.ps1 -Scenario 03-workload -SkipStartup
```

### Available Scenarios

| Scenario | Description |
|----------|-------------|
| `01-simple` | Basic SPIRE server and agent with join token attestation |
| `02-tornjak` | Tornjak management UI and API for SPIRE |
| `03-workload` | Workload identity with Unix attestor |
| `04-tpm` | TPM-style node attestation (certificate-based stand-in) |
| `05-svid-api` | mTLS between Go services using SVIDs |
| `06-metrics` | SPIRE telemetry with Prometheus, Graphite, and Tornjak |
| `07-production` | Production-like combined platform scenario |

### What It Tests

For each scenario, the smoke test checks:

1. **Prerequisites** — Podman CLI availability
2. **Scenario Directory** — compose.yml present
3. **Running Scenario Detection** — Warns if containers from another scenario are running
4. **Compose Stack** — Starts containers using the scenario's `env-up.ps1` script
5. **Container Status** — Verifies all required containers are running
6. **Health Checks** — Scenario-specific endpoint checks (for example Prometheus and Graphite)
7. **Configuration Files** — Verifies SPIRE and Tornjak configs exist
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
