# Scenario 05 — SVID-Based Service API (mTLS)

> **Complexity:** Intermediate-Advanced · **Previous:** [04-tpm](../04-tpm/README.md) · **Next:** [06-metrics](../06-metrics/README.md)

## What You Will Learn
- How to use SVIDs programmatically with the go-spiffe SDK
- Mutual TLS (mTLS) — both sides verify each other's identity
- The Workload API — how services get certificates without managing them
- Certificate rotation — zero-downtime cert renewal
- SPIFFE ID authorization — validating who the peer is

## Background Concepts

### Why Programmatic SVIDs?
In earlier SPIFFE/SPIRE exercises it is common to inspect SVIDs with CLI tools. That is useful for learning, but real applications do not shell out to fetch certificates. A service needs a library that can connect to SPIRE, ask for its identity, keep that identity fresh, and hand it to the TLS stack automatically.

That is the problem solved by the go-spiffe SDK. In this scenario the client and server never read a certificate file from disk, never generate their own key pair, and never reload configuration when a certificate changes. They simply connect to the SPIRE Agent's Workload API and let the SDK manage identity.

### Mutual TLS (mTLS)
Standard TLS proves the server's identity to the client. The browser checks the website certificate, but the website usually does not check a certificate from the browser.

mTLS adds the missing half: the client also presents a certificate, and the server verifies it. In a SPIFFE-based system, both certificates are X.509-SVIDs. That means the important identity value is the SPIFFE ID inside the certificate's SAN URI field.

The result is stronger than "the connection is encrypted":
1. The traffic is encrypted.
2. The client knows exactly which server it reached.
3. The server knows exactly which client called it.
4. Both identities come from a shared trust system instead of ad-hoc secrets.

### The Workload API
The Workload API is the bridge between an application and SPIRE. A workload connects to the local SPIRE Agent over a Unix domain socket and requests identity material. The agent verifies which workload is asking, checks registration entries, and returns the correct SVIDs and trust bundles.

In this scenario the socket lives at `/opt/spire/sockets/workload_api.sock` inside the containers. The Go applications connect to that socket through `workloadapi.NewX509Source(...)`.

### The go-spiffe SDK
`github.com/spiffe/go-spiffe/v2` is the official Go SDK for SPIFFE. The key pieces used here are:

- **X509Source** — keeps a live view of the workload's X.509-SVID and trust bundle.
- **tlsconfig** — builds `tls.Config` values that know how to present SVIDs and verify peers.
- **spiffeid** — parses, validates, and compares SPIFFE IDs and trust domains.
- **workloadapi** — connects to the Workload API socket.

A good mental model is: `workloadapi` gets identity from SPIRE, `X509Source` keeps it current, and `tlsconfig` plugs it into standard Go networking.

### Certificate Rotation
SVIDs are intentionally short-lived. That reduces blast radius if a certificate is exposed and avoids the operational pain of long-lived credentials. The important point is that short lifetime does **not** mean manual renewal.

`X509Source` watches the Workload API stream. When SPIRE rotates the certificate, the source updates automatically. New TLS handshakes use the new certificate without restarting the process. This is one of the biggest practical wins of SPIFFE-based identity.

### Authorization with SPIFFE IDs
Authentication answers **who are you?** Authorization answers **are you allowed to do this?**

After the TLS handshake completes, the peer certificate has already been validated against the trust bundle. At that point your application can apply authorization rules based on the peer SPIFFE ID.

The demo uses `AuthorizeMemberOf(td)`, which means "accept any workload in the `mirmat.org` trust domain." That keeps the first example easy to understand. In real systems you often narrow this further:

- `AuthorizeID(id)` — allow exactly one workload identity.
- `AuthorizeOneOf(...)` — allow a small set of identities.
- Custom logic — allow by path prefix, service role, or policy engine decision.

## Architecture
```text
┌──────────────────┐     mTLS      ┌──────────────────┐
│   svid-client    │──────────────►│   svid-server    │
│                  │               │                  │
│ SPIFFE ID:       │               │ SPIFFE ID:       │
│ spiffe://mirmat  │               │ spiffe://mirmat  │
│ .org/svid-client │               │ .org/svid-server │
└────────┬─────────┘               └────────┬─────────┘
         │                                  │
         │     Workload API (Unix socket)   │
         └──────────┬───────────────────────┘
                    │
         ┌──────────▼─────────┐
         │   SPIRE Agent      │
         │                    │
         │ Issues SVIDs to    │
         │ registered         │
         │ workloads          │
         └──────────┬─────────┘
                    │ attestation
         ┌──────────▼─────────┐
         │   SPIRE Server     │
         │                    │
         │ Trust domain:      │
         │ mirmat.org         │
         └────────────────────┘
```

## Scenario Files
- `server/main.go` — HTTPS server using an X.509-SVID as its TLS certificate.
- `client/main.go` — mTLS client that repeatedly calls the server.
- `spire/server/server.conf` — SPIRE Server with `join_token` node attestation.
- `spire/agent/agent.conf` — SPIRE Agent exposing the Workload API socket.
- `compose.yml` — container topology for SPIRE plus the two Go services.
- `scripts/*.ps1` — helper scripts to build, start, register, and clean up the demo.

## Code Walkthrough

### Server: `server/main.go`
The server demonstrates the most important production pattern: create an `X509Source`, use it to build a TLS configuration, and let the library keep certificates fresh.

#### 1. Connect to the Workload API
```go
x509Source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
```
This opens a connection to the SPIRE Agent over the Unix socket. The returned source does more than fetch one certificate once. It stays connected and tracks updates.

#### 2. Read the workload's own identity
```go
svid, err := x509Source.GetX509SVID()
log.Printf("Server SPIFFE ID: %s", svid.ID)
```
The server logs its SPIFFE ID so you can see exactly which identity SPIRE issued. This is useful when teaching because it makes the abstract certificate exchange visible.

#### 3. Define the trust domain
```go
td, err := spiffeid.TrustDomainFromString("mirmat.org")
```
A trust domain is the administrative boundary of SPIFFE identities. `spiffe://mirmat.org/...` means the identity belongs to the `mirmat.org` trust domain.

#### 4. Build an mTLS server config
```go
tlsConfig := tlsconfig.MTLSServerConfig(x509Source, x509Source, tlsconfig.AuthorizeMemberOf(td))
```
This single line is the heart of the scenario:
- the first `x509Source` supplies the server certificate,
- the second `x509Source` supplies trust bundles for peer verification,
- `AuthorizeMemberOf(td)` accepts only clients from the expected trust domain.

The server does **not** load PEM files or implement custom certificate validation logic. The SDK handles that for you.

#### 5. Read the peer identity from the TLS connection
Inside the `/` handler the code inspects `r.TLS.PeerCertificates` and extracts the first SPIFFE URI. Once the mTLS handshake succeeds, that URI tells the server exactly which workload called it.

That is a key teaching point: after mTLS, identity becomes application data. You can log it, authorize on it, or attach it to audit records.

#### 6. Start a normal Go HTTP server
```go
server := &http.Server{
    Addr:      ":" + port,
    Handler:   mux,
    TLSConfig: tlsConfig,
}
```
Other than the SPIFFE-based TLS configuration, this is just the standard `net/http` server you already know. SPIFFE integrates into normal application code instead of requiring a completely different programming model.

### Client: `client/main.go`
The client mirrors the same pattern from the other side of the connection.

#### 1. Create an `X509Source`
The client also connects to the Workload API and receives its own SVID plus trust bundle data.

#### 2. Log its own SPIFFE ID
```go
log.Printf("Client SPIFFE ID: %s", svid.ID)
```
This confirms that the client has an identity independent from the server.

#### 3. Build an mTLS client config
```go
tlsConfig := tlsconfig.MTLSClientConfig(x509Source, x509Source, tlsconfig.AuthorizeMemberOf(td))
```
The client presents its own SVID and requires the server to prove an identity in the same trust domain.

#### 4. Reuse standard HTTP client code
```go
client := &http.Client{
    Transport: &http.Transport{TLSClientConfig: tlsConfig},
}
```
Again, the only special part is the TLS configuration. The rest is ordinary Go HTTP code.

#### 5. Call the server in a loop
The loop sends five requests with pauses between them. That gives you time to watch logs and observe repeated identity-aware communication. In a longer-running program, the same pattern would continue through certificate rotations.

## Why the Containers Use Different UIDs
Both workloads are attested with the SPIRE Agent's `unix` workload attestor. That attestor can produce selectors such as `unix:uid:<number>`. If both containers ran as the same Unix user, they could match the same registration entries and receive the wrong identity material.

To keep the demo deterministic:
- the server container runs as UID `10001`,
- the client container runs as UID `10002`,
- the registration script uses matching `unix:uid` selectors.

This is still a teaching shortcut. Production systems usually combine several selectors or use more workload-specific attestation signals.

## Prerequisites
- Podman and podman-compose
- PowerShell 7+
- Go 1.26+ (for local development; not needed if using containers)

## Running the Scenario
From `scenarios/05-svid-api`, run:

```powershell
.\scripts\env-up.ps1
```

The script performs these steps for you:
1. Builds the Go client and server images.
2. Starts the SPIRE Server.
3. Generates a join token.
4. Starts the SPIRE Agent with that join token.
5. Registers workload entries for the server and client.
6. Starts the two Go services.

## What to Observe
- The server logs its own SPIFFE ID when it starts.
- The client logs its own SPIFFE ID when it starts.
- The server logs the client's SPIFFE ID on every request.
- The client prints the response body showing both identities.
- No certificate files are mounted into the application containers.
- The applications use ordinary Go HTTP code once the TLS config is built.

## Useful Commands
```powershell
podman-compose logs -f svid-server
podman-compose logs -f svid-client
podman-compose logs -f spire-agent
podman-compose logs -f spire-server
```

## Suggested Experiments
1. Change `AuthorizeMemberOf(td)` to a stricter rule such as `AuthorizeID(...)`.
2. Add a second server endpoint and allow only a specific client identity to reach it.
3. Reduce SVID lifetime in SPIRE and observe that new connections continue working after rotation.
4. Print more certificate details from the peer connection to reinforce how SPIFFE information is carried in X.509.

## Troubleshooting
- **Client cannot connect to server** — check `podman-compose logs -f spire-agent` and confirm both workloads received SVIDs.
- **No attested agent found** — wait a few more seconds after starting the agent, then rerun `register-workloads.ps1`.
- **Wrong workload identity** — verify that the container UID in each Containerfile matches the selector in `register-workloads.ps1`.
- **Build fails with missing modules** — run `go mod tidy` at the repository root to refresh `go.sum`.


## Tornjak UI

Tornjak is co-deployed as a management UI alongside the SPIRE server. Once the scenario is running, open your browser at:

- **Tornjak UI**: http://localhost:3000
- **Tornjak API**: http://localhost:10000

From the UI you can:
- View all attested agents (Agents tab)
- Browse and create workload registration entries (Entries tab)
- Inspect the trust bundle (Trust Domains tab)

This makes the SPIFFE/SPIRE configuration visible without requiring CLI commands.

## Cleanup
```powershell
.\scripts\env-down.ps1
```

---
**Next:** [06-metrics](../06-metrics/README.md) — Monitor SPIRE with Prometheus and Graphite
