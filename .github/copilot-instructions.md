# Copilot Instructions

## Repository Overview

This is a hands-on learning demo for [SPIFFE](https://spiffe.io/) and [SPIRE](https://spiffe.io/docs/latest/spire-about/). It now contains seven self-contained, progressively complex scenarios. The only Go code lives in `scenarios/05-svid-api/`, which demonstrates two services communicating via mTLS using X.509 SVIDs.

## Build, Lint, and Test

Uses [Task](https://taskfile.dev/) for Go tasks. All commands run from the repository root.

```powershell
task build               # Build both Go binaries (server + client)
task build-svid-server   # Build server only
task build-svid-client   # Build client only
task lint                # go vet ./...
task test                # go test ./...
go test ./scenarios/05-svid-api/...  # Run tests for a single package
```

Run smoke tests for any scenario:

```powershell
.\scripts\smoke-test.ps1 -Scenario 01-simple
.\scripts\smoke-test.ps1 -Scenario 05-svid-api -Verbose
.\scripts\smoke-test.ps1 -Scenario 03-workload -SkipStartup  # containers already running
```

Valid scenario names: `01-simple`, `02-tornjak`, `03-workload`, `04-tpm`, `05-svid-api`, `06-metrics`, `07-production`.

## Architecture

### Scenario Layout

Each scenario is a fully self-contained `podman-compose` stack in `scenarios/<N>-<name>/`:

```
scenarios/<N>-<name>/
├── compose.yml
├── spire/
│   ├── server/server.conf
│   └── agent/agent.conf
└── scripts/
    ├── env-up.ps1       # Start the stack
    └── env-down.ps1     # Tear down the stack
```

Scenario 05 also contains Go source in `server/` and `client/` subdirectories. Scenario 07 adds a second agent, Tornjak, and telemetry collectors.

### How Scenarios Start

**Always use `scripts/env-up.ps1` instead of `podman-compose up -d` directly.** The helper scripts handle runtime-specific startup such as join token generation, credential provisioning, workload registration, and Tornjak readiness checks.

Only one scenario should be running at a time — containers share names and host ports that can conflict.

### Go Services (Scenario 05)

The `svid-server` and `svid-client` binaries use [go-spiffe v2](https://github.com/spiffe/go-spiffe). Both connect to the Workload API at:

```
unix:///opt/spire/sockets/workload_api.sock
```

The socket is shared between SPIRE agent and Go containers via a named volume (`workload-socket`).

**Container build context is the repository root**, not the scenario directory, so `go.mod`/`go.sum` are accessible. The Containerfiles are at `scenarios/05-svid-api/{server,client}/Containerfile`.

Workloads are authorized by unix UID selectors:
- `svid-server` runs as UID `10001` → `spiffe://mirmat.org/svid-server`
- `svid-client` runs as UID `10002` → `spiffe://mirmat.org/svid-client`

Workload registration happens in `scripts/register-workloads.ps1` using `spire-server entry create`.

## Key Conventions

- **Trust domain** is `mirmat.org` across all scenarios.
- **SPIRE version** is `1.14.5` (pinned in all `compose.yml` files).
- `insecure_bootstrap = true` is intentional in agent configs for join-token demos, not for production.
- Tornjak uses `ghcr.io/spiffe/tornjak-backend:v1.2.2` and `ghcr.io/spiffe/tornjak-frontend:v1.2.2` wherever it appears.
- Server configs use `KeyManager "memory"` and SQLite (`DataStore "sql"`) unless the scenario is intentionally demonstrating a different pattern.
