# SPIFFE/SPIRE Demo

A hands-on learning guide for [SPIFFE](https://spiffe.io/) (Secure Production Identity Framework for Everyone) and [SPIRE](https://spiffe.io/docs/latest/spire-about/) (SPIFFE Runtime Environment). Each scenario builds on the previous one, introducing new concepts progressively.

## What is SPIFFE?

SPIFFE provides a secure identity framework for services in dynamic environments. Instead of relying on network-level security (IP addresses, firewalls), SPIFFE gives each workload a cryptographic identity -- an **SVID** (SPIFFE Verifiable Identity Document).

SPIRE is the reference implementation of SPIFFE, handling identity issuance, attestation, and rotation.

## Scenarios

Work through the scenarios in order -- each builds on concepts from the previous one.

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
- [Go 1.26+](https://go.dev/dl/) (for building scenario binaries and services)
- [Task](https://taskfile.dev/) (optional, for building Go code locally)

## Quick Start

```bash
# Build all scenario binaries
task build

# Run the simplest scenario
./bin/scenario-01-simple up

# Run with guided learning mode (pauses between steps)
./bin/scenario-01-simple step

# Verbose output for debugging
./bin/scenario-01-simple up --verbose

# Tear down when done
./bin/scenario-01-simple down
```

## Building

```bash
# Build everything (scenarios + dashboard + services)
task build

# Build just scenarios
task build-scenarios

# Build individual binaries
go build -o bin/scenario-01-simple ./cmd/scenario-01-simple/
```

## Guided Learning Mode

All scenario binaries support the `step` command for interactive learning. In this mode,
each step pauses after printing an explanation, giving you time to understand what
is happening before continuing. Press Enter to proceed to the next step.

```bash
./bin/scenario-05-svid-api step
```

## Project Structure

```
cmd/
  scenario-01-simple/      Scenario binaries (one per scenario)
  scenario-02-dashboard/
  ...
  scenario-07-production/
  dashboard/               SPIRE dashboard web server
  svid-server/             mTLS demo server (scenario 05)
  svid-client/             mTLS demo client (scenario 05)
internal/
  logging/                 Colored terminal output
  scenario/                Unified CLI framework for scenario binaries
  step/                    Interactive step runner framework
  podman/                  Typed wrapper for podman/podman-compose
  spirectl/                SPIRE server CLI helpers
  certs/                   x509 certificate generation
scenarios/
  01-simple/               Compose files, SPIRE configs, Containerfiles
  ...
  07-production/
Taskfile.yml               Build tasks
go.mod                     Go module definition
```
