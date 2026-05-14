# SPIFFE/SPIRE Demo

A hands-on learning guide for [SPIFFE](https://spiffe.io/) (Secure Production Identity Framework for Everyone) and [SPIRE](https://spiffe.io/docs/latest/spire-about/) (SPIFFE Runtime Environment). Each scenario builds on the previous one, introducing new concepts progressively.

## What is SPIFFE?

SPIFFE provides a secure identity framework for services in dynamic environments. Instead of relying on network-level security (IP addresses, firewalls), SPIFFE gives each workload a cryptographic identity — an **SVID** (SPIFFE Verifiable Identity Document).

SPIRE is the reference implementation of SPIFFE, handling identity issuance, attestation, and rotation.

## Scenarios

Work through the scenarios in order — each builds on concepts from the previous one.

| # | Scenario | Concepts | Description |
|---|----------|----------|-------------|
| 1 | [01-simple](scenarios/01-simple/README.md) | Trust domains, SPIFFE IDs, node attestation | Minimal SPIRE server and agent with join token attestation |
| 2 | [02-dashboard](scenarios/02-dashboard/README.md) | SPIRE dashboard, server API socket, gRPC | Add a read-only dashboard to visualize agents, entries, and trust bundles |
| 3 | [03-workload](scenarios/03-workload/README.md) | Workload registration, Unix attestor, SVID lifecycle | Add a workload container and fetch its X.509 SVID |
| 4 | [04-tpm](scenarios/04-tpm/README.md) | TPM concepts, hardware identity, certificate-based attestation | Certificate-based node attestation (simulating TPM DevID) |
| 5 | [05-svid-api](scenarios/05-svid-api/README.md) | go-spiffe SDK, mTLS, certificate rotation | Two Go services communicating via mTLS using SVIDs |
| 6 | [06-metrics](scenarios/06-metrics/README.md) | Telemetry, Prometheus, StatsD, Graphite, dashboard | Monitor SPIRE while exploring it through the dashboard |
| 7 | [07-production](scenarios/07-production/README.md) | Mixed attestation, multi-agent SPIRE, management, telemetry | Combine workloads, dashboard, and metrics into a production-like lab |

## Prerequisites

- [Podman](https://podman.io/getting-started/installation) and [podman-compose](https://github.com/containers/podman-compose)
- [PowerShell 7+](https://learn.microsoft.com/en-us/powershell/scripting/install/installing-powershell)
- [Go 1.26+](https://go.dev/dl/) (for scenario 05-svid-api and dashboard local development)
- [Task](https://taskfile.dev/) (optional, for building Go code locally)

## Quick Start

```powershell
# Start with the simplest scenario
cd scenarios/01-simple
.\scripts\env-up.ps1

# When done, clean up
.\scripts\env-down.ps1
```

## Smoke Tests

Run automated tests for any scenario:

```powershell
.\scripts\smoke-test.ps1 -Scenario 01-simple
```

See [scripts/README.md](scripts/README.md) for full documentation.

## Project Structure

```
├── scenarios/
│   ├── 01-simple/          # Beginner: minimal SPIRE setup
│   ├── 02-dashboard/        # Beginner: read-only SPIRE dashboard
│   ├── 03-workload/        # Beginner-Intermediate: workload identity
│   ├── 04-tpm/             # Intermediate: TPM-style node attestation
│   ├── 05-svid-api/        # Intermediate-Advanced: mTLS with Go
│   ├── 06-metrics/         # Advanced: telemetry, monitoring, and dashboard
│   └── 07-production/      # Advanced: combined production-like platform
├── scripts/
│   ├── smoke-test.ps1      # Scenario-aware smoke tests
│   └── README.md
├── Taskfile.yml            # Go build tasks
├── go.mod                  # Go module definition
└── README.md               # This file
```
