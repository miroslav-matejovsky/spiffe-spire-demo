# Copilot Instructions

## Repository Overview

This is a hands-on learning demo for [SPIFFE](https://spiffe.io/) and [SPIRE](https://spiffe.io/docs/latest/spire-about/). It contains five self-contained, progressively complex scenarios. The only Go code lives in `scenarios/04-svid-api/`, which demonstrates two services communicating via mTLS using X.509 SVIDs.

## Build, Lint, and Test

Uses [Task](https://taskfile.dev/) for Go tasks. All commands run from the repository root.

```powershell
task build               # Build both Go binaries (server + client)
task build-svid-server   # Build server only
task build-svid-client   # Build client only
task lint                # go vet ./...
task test                # go test ./...
go test ./scenarios/04-svid-api/...  # Run tests for a single package
```

Run smoke tests for any scenario:

```powershell
.\scripts\smoke-test.ps1 -Scenario 01-simple
.\scripts\smoke-test.ps1 -Scenario 04-svid-api -Verbose
.\scripts\smoke-test.ps1 -Scenario 02-workload -SkipStartup  # containers already running
```

Valid scenario names: `01-simple`, `02-workload`, `03-tpm`, `04-svid-api`, `05-metrics`.

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
    ├── set-env.ps1      # Start the stack
    └── clean-env.ps1    # Tear down the stack
```

Scenario 04 also contains Go source in `server/` and `client/` subdirectories.

### How Scenarios Start

**Always use `scripts/set-env.ps1` instead of `podman-compose up -d` directly.** Every scenario uses join token attestation: the agent requires a freshly generated one-time token from the server at startup. The `set-env.ps1` scripts:
1. Start the SPIRE server and wait for it to become ready
2. Generate a join token via `spire-server token generate`
3. Start the SPIRE agent with `-joinToken <token>`
4. For scenario 04: also build container images and register workload entries

Only one scenario should be running at a time — containers share names that can conflict.

### Go Services (Scenario 04)

The `svid-server` and `svid-client` binaries use [go-spiffe v2](https://github.com/spiffe/go-spiffe). Both connect to the Workload API at:

```
unix:///opt/spire/sockets/workload_api.sock
```

The socket is shared between SPIRE agent and Go containers via a named volume (`workload-socket`).

**Container build context is the repository root**, not the scenario directory, so `go.mod`/`go.sum` are accessible. The Containerfiles are at `scenarios/04-svid-api/{server,client}/Containerfile`.

Workloads are authorized by unix UID selectors:
- `svid-server` runs as UID `10001` → `spiffe://mirmat.org/svid-server`
- `svid-client` runs as UID `10002` → `spiffe://mirmat.org/svid-client`

Workload registration happens in `scripts/register-workloads.ps1` using `spire-server entry create`.

## Key Conventions

- **Trust domain** is `mirmat.org` across all scenarios.
- **SPIRE version** is `1.14.5` (pinned in all `compose.yml` files).
- `insecure_bootstrap = true` is intentional in agent configs — required for join token demos, not for production.
- Server configs use `KeyManager "memory"` and SQLite (`DataStore "sql"`) — data does not persist across restarts.
- The SPIRE agent in all scenarios is started via `podman-compose up -d spire-agent` after setting `$env:SPIRE_AGENT_JOIN_TOKEN`. Container names follow the pattern `<scenario-folder-name>_<service>_1` for all services.
