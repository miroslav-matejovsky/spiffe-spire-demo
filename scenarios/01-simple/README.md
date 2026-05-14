# Scenario 01 — Simple SPIRE Setup

> **Complexity:** Beginner · **Next:** [02-workload](../02-workload/README.md)

## What You Will Learn

- What SPIFFE is and why it exists
- Core concepts: trust domains, SPIFFE IDs, SVIDs
- How node attestation works
- Setting up a minimal SPIRE Server and Agent
- Why join tokens and insecure bootstrap are acceptable for local demos only

## Background Concepts

### What is SPIFFE?

SPIFFE (Secure Production Identity Framework for Everyone) is an open standard for securely identifying software services in dynamic and heterogeneous environments. Instead of relying on network-level identity such as IP addresses or firewall rules, SPIFFE gives workloads cryptographic identities.

### Trust Domains

A trust domain is the root of a SPIFFE identity system. All identities inside the same trust domain share a common root of trust. The trust domain is represented as a hostname-like string, for example `mirmat.org`.

### SPIFFE IDs

A SPIFFE ID is a URI that uniquely identifies a workload or node.

Format:
`spiffe://trust-domain/path`

Example:
`spiffe://mirmat.org/myagent`

### SVIDs (SPIFFE Verifiable Identity Documents)

An SVID is the document that carries the SPIFFE ID.

- **X.509-SVID**: An X.509 certificate with the SPIFFE ID encoded in the SAN URI field
- **JWT-SVID**: A JWT with the SPIFFE ID in the `sub` claim

### SPIRE Architecture

SPIRE is a production-ready implementation of the SPIFFE standards. It has two core components:

- **SPIRE Server**: The central authority that manages trust, performs node attestation, and signs SVIDs
- **SPIRE Agent**: Runs on a node, attests to the server, and exposes the Workload API to local workloads

### Node Attestation

Before an agent can receive identities for workloads, it must first prove to the server that it is an authorized node. This is called **node attestation**.

In this scenario we use the simplest mechanism: **join tokens**. The server generates a one-time secret, and the agent presents that secret during startup.

### Why `insecure_bootstrap = true`?

The first time the agent talks to the server, it does not yet have a trust bundle. Setting `insecure_bootstrap = true` allows the initial bootstrap to happen without pre-distributing that bundle.

This is convenient for learning and demos, but it is **not appropriate for production**.

## Architecture Diagram

```text
┌─────────────────────┐     attestation     ┌─────────────────────┐
│   SPIRE Server      │◄──────────────────►│   SPIRE Agent       │
│                     │    (join token)     │                     │
│ - Trust domain root │                     │ - Attests to server │
│ - Signs SVIDs       │                     │ - Workload API      │
│ - Manages entries   │                     │                     │
└─────────────────────┘                     └─────────────────────┘
```

## Prerequisites

- Windows with Podman and `podman-compose` installed
- PowerShell 7+

## Files in This Scenario

- `compose.yml` defines the SPIRE Server and SPIRE Agent containers
- `spire/server/server.conf` configures the SPIRE Server
- `spire/agent/agent.conf` configures the SPIRE Agent
- `scripts/set-env.ps1` starts the server, creates a join token, and launches the agent with that token
- `scripts/clean-env.ps1` stops and removes the demo containers

## Running the Scenario

### Start the Environment

```powershell
cd scenarios/01-simple
.\scripts\set-env.ps1
```

Use the helper script instead of `podman-compose up -d` directly, because the agent needs a freshly generated join token at startup.

The script performs these steps:

1. Starts the SPIRE Server
2. Waits for the server to become ready
3. Generates a one-time join token
4. Starts the SPIRE Agent with that token
5. Verifies that the agent attested successfully

### Verify Agent Attestation

```powershell
podman-compose exec spire-server /opt/spire/bin/spire-server agent list
```

You should see an attested agent in the `mirmat.org` trust domain with attestation type `join_token`, typically something like `spiffe://mirmat.org/spire/agent/join_token/<uuid>`.

### View Logs

Because the helper script launches the agent with a runtime join token, the server and agent logs are easiest to inspect separately:

```powershell
podman-compose logs -f spire-server
podman logs -f spire-simple-agent
```

### What Happened?

1. The SPIRE Server started and initialized the trust domain `mirmat.org`
2. The server generated a one-time join token for the demo
3. The SPIRE Agent started with `-joinToken <token>`
4. The server validated the token and attested the agent using the `join_token` node attestor
5. The agent is now trusted as a node in the `mirmat.org` trust domain

## Cleanup

```powershell
.\scripts\clean-env.ps1
```

---

**Next:** [02-workload](../02-workload/README.md) — Learn how workloads get their own identities
