package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "SVID API",
		DirName: "05-svid-api",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	err := ctx.Runner.RunStep(step.Step{
		Name: "Building Container Images",
		Explain: "Three images get built for one full mTLS lab.\n" +
			"Dashboard gives SPIRE state and registration views.\n" +
			"svid-server is Go HTTPS service using workloadapi.NewX509Source for server certs.\n" +
			"svid-client is Go HTTP client using same API for client certs.\n" +
			"Neither service reads PEM files from disk. go-spiffe/v2 fetches and rotates SVIDs at runtime.",
		Sources: []step.Source{
			ctx.Src("server/Containerfile", 10, "UID 10001 for svid-server"),
			ctx.Src("client/Containerfile", 10, "UID 10002 for svid-client"),
			ctx.RepoSrc("cmd/svid-server/main.go", 24, "workloadapi.NewX509Source"),
			ctx.RepoSrc("cmd/svid-client/main.go", 25, "workloadapi.NewX509Source"),
			ctx.RepoSrc("dashboard/Containerfile", 0, "dashboard multi-stage build"),
		},
		Action: func() error {
			serverCf := filepath.Join(ctx.ScenarioDir, "server", "Containerfile")
			clientCf := filepath.Join(ctx.ScenarioDir, "client", "Containerfile")

			if err := podman.BuildDashboard(ctx.RepoRoot, ctx.Log); err != nil {
				return err
			}
			if err := podman.Build("spiffe-spire-demo-svid-server:local", serverCf, ctx.RepoRoot, ctx.Log); err != nil {
				return err
			}
			return podman.Build("spiffe-spire-demo-svid-client:local", clientCf, ctx.RepoRoot, ctx.Log)
		},
		Observe: "Three images now sit in local container cache.\n" +
			"Go services are wired to go-spiffe/v2, so cert material comes from SPIRE, not image files.\n" +
			"Check Containerfiles if curious: app code ships, but no static server.crt or client.key.\n" +
			"Next steps bring up SPIRE so those images can ask for live SVIDs.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server",
		Explain: "SPIRE server is trust authority for trust domain mirmat.org.\n" +
			"It stores registration entries that bind workloads to SPIFFE IDs.\n" +
			"Later it will issue X.509 SVIDs for svid-server and svid-client.\n" +
			"Agent and dashboard both depend on this control plane.",
		Sources: []step.Source{
			ctx.Src("spire/server/server.conf", 5, "trust_domain = mirmat.org"),
			ctx.Src("spire/server/server.conf", 17, "NodeAttestor join_token"),
			ctx.Src("compose.yml", 4, "spire-server service"),
		},
		Action: func() error {
			return ctx.Compose.Up("spire-server")
		},
		Observe: "SPIRE server container is up.\n" +
			"Server API can now accept agent join, workload entry creation, and bundle reads.\n" +
			"Next step waits until health check says CA and datastore are ready.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for SPIRE Server Health",
		Explain: "Start command only starts process.\n" +
			"Health check waits until server API really answers.\n" +
			"No agent or workload should depend on server before this point.",
		Action: func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "SPIRE server is healthy now.\n" +
			"CA is ready to sign SVIDs and publish trust bundle data.\n" +
			"Safe point for agent startup and workload registration.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Agent",
		Explain: "SPIRE agent is node-side bridge between workloads and SPIRE server.\n" +
			"Compose shares workload-socket volume among spire-agent, svid-server, and svid-client.\n" +
			"Both Go apps call unix:///opt/spire/sockets/workload_api.sock through workloadapi.NewX509Source.\n" +
			"Agent uses join token once, then serves SVID and bundle updates locally.",
		Sources: []step.Source{
			ctx.Src("spire/agent/agent.conf", 6, "socket_path for Workload API"),
			ctx.Src("compose.yml", 20, "workload-socket volume on agent"),
			ctx.Src("compose.yml", 32, "workload-socket volume on svid-server"),
			ctx.Src("compose.yml", 45, "workload-socket volume on svid-client"),
		},
		Action: func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_JOIN_TOKEN", token)
			defer podman.UnsetEnv("SPIRE_AGENT_JOIN_TOKEN")
			return ctx.Compose.Up("spire-agent")
		},
		Observe: "Agent container started with fresh join token.\n" +
			"After attestation, Workload API socket at /opt/spire/sockets/workload_api.sock will be live.\n" +
			"Both Go services mount same socket path through workload-socket volume.\n" +
			"Apps never talk to SPIRE server direct. Apps talk to local agent.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Agent Attestation",
		Explain: "Join token proves agent is allowed into trust domain.\n" +
			"Server records agent identity after attestation succeeds.\n" +
			"Only then can agent answer Workload API calls for local processes.",
		Action: func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
		Observe: "Agent attested with SPIRE server.\n" +
			"Node now has authenticated channel for SVID and bundle updates.\n" +
			"Workload API is ready to serve both Go services.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Registering Workloads",
		Explain: "Registration binds Unix process identity to SPIFFE identity.\n" +
			"SPIRE unix workload attestor checks UID of process opening Workload API socket.\n" +
			"svid-server runs as UID 10001, so unix:uid:10001 maps to spiffe://mirmat.org/svid-server.\n" +
			"svid-client runs as UID 10002, so unix:uid:10002 maps to spiffe://mirmat.org/svid-client.\n" +
			"Different UIDs mean different SVIDs and clean authorization boundary.\n" +
			"Same socket and same agent still yield per-process identity.",
		Sources: []step.Source{
			ctx.Src("server/Containerfile", 12, "USER 10001 for svid-server"),
			ctx.Src("client/Containerfile", 12, "USER 10002 for svid-client"),
			ctx.RepoSrc("internal/spirectl/spirectl.go", 97, "CreateEntry implementation"),
		},
		Action: func() error {
			agentID, err := spirectl.GetAgentID(ctx.Compose, "spire-server", ctx.Log)
			if err != nil {
				return err
			}
			if err := spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/svid-server", agentID, "unix:uid:10001", ctx.Log,
				"mTLS demo server - handles incoming requests"); err != nil {
				return err
			}
			return spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/svid-client", agentID, "unix:uid:10002", ctx.Log,
				"mTLS demo client - sends periodic requests")
		},
		Observe: "Two workload entries now exist on SPIRE server.\n" +
			"When svid-server with UID 10001 calls Workload API, agent returns spiffe://mirmat.org/svid-server.\n" +
			"When svid-client with UID 10002 calls Workload API, agent returns spiffe://mirmat.org/svid-client.\n" +
			"Each service now has its own cryptographic identity for mTLS and authorization.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting mTLS Services",
		Explain: "1) svid-server calls workloadapi.NewX509Source() and gets rotating SVID data.\n" +
			"2) It builds tls.Config with tlsconfig.MTLSServerConfig() and tlsconfig.AuthorizeMemberOf().\n" +
			"3) It listens on :8443 without reading PEM files from disk.\n" +
			"4) svid-client creates its own X509Source and tlsconfig.MTLSClientConfig().\n" +
			"5) On connect, both sides present X.509 SVIDs and verify peer against trust bundle.\n" +
			"6) Server reads client SPIFFE ID from peer cert URI SAN after handshake.\n" +
			"7) X509Source watches updates, so certificate rotation can happen without app restart.",
		Sources: []step.Source{
			ctx.RepoSrc("cmd/svid-server/main.go", 24, "NewX509Source connects to Workload API"),
			ctx.RepoSrc("cmd/svid-server/main.go", 47, "MTLSServerConfig with AuthorizeMemberOf"),
			ctx.RepoSrc("cmd/svid-server/main.go", 54, "extract client SPIFFE ID from peer cert"),
			ctx.RepoSrc("cmd/svid-client/main.go", 25, "NewX509Source for client identity"),
			ctx.RepoSrc("cmd/svid-client/main.go", 43, "MTLSClientConfig for mTLS dial"),
		},
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("svid-server"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return ctx.Compose.UpNoBuild("svid-client")
		},
		Observe: "Both services are starting now.\n" +
			"Watch svid-server logs for server SPIFFE ID and client SPIFFE ID seen after each handshake.\n" +
			"Watch svid-client logs for HTTPS responses proving both identities were accepted.\n" +
			"Try: " + ctx.ComposeCmd() + " logs -f svid-server\n" +
			"Try: " + ctx.ComposeCmd() + " logs -f svid-client",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: "Dashboard is read-only window into SPIRE server state.\n" +
			"Use it to inspect entries, agents, and health while mTLS traffic runs.\n" +
			"It helps link control-plane objects to live app behavior.",
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
		Observe: "Dashboard is live at http://localhost:8080.\n" +
			"Check Entries page for svid-server and svid-client workload registrations.\n" +
			"Check Agents page for attested agent that serves both workloads.\n" +
			"Keep logs open beside dashboard to connect control plane with mTLS traffic.",
	})
	if err != nil {
		return err
	}

	return nil
}

func done(ctx *scenario.Context) {
	composeCmd := ctx.ComposeCmd()
	ctx.Log.Step("SVID API scenario is ready!")
	ctx.Log.Info("Dashboard:    http://localhost:8080")
	ctx.Log.Info("Server logs:  " + composeCmd + " logs -f svid-server")
	ctx.Log.Info("Client logs:  " + composeCmd + " logs -f svid-client")
	ctx.Log.Info("")
	ctx.Log.Info("What is happening:")
	ctx.Log.Info("  - svid-server has SPIFFE ID: spiffe://mirmat.org/svid-server")
	ctx.Log.Info("  - svid-client has SPIFFE ID: spiffe://mirmat.org/svid-client")
	ctx.Log.Info("  - They communicate over mTLS using auto-rotated X.509 SVIDs")
	ctx.Log.Info("  - No certificates were manually configured in the applications")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-05-svid-api down")
}
