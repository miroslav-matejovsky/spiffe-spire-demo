package main

import (
	"fmt"
	"path/filepath"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "Production",
		DirName: "07-production",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	agent2CertPath := filepath.Join(ctx.ScenarioDir, "spire", "agent-2", "devid-cert.pem")
	agent2KeyPath := filepath.Join(ctx.ScenarioDir, "spire", "agent-2", "devid-key.pem")
	serverCaPath := filepath.Join(ctx.ScenarioDir, "spire", "server", "devid-ca.pem")

	err := ctx.Runner.RunStep(step.Step{
		Name: "Provisioning DevID Credentials for Agent-2",
		Explain: "Scenario 07 mixes node attestation methods inside one trust domain.\n" +
			"Agent-1 will use join_token, like cloud VM or dev box with no hardware root.\n" +
			"Agent-2 will use x509pop with DevID cert, like bare metal or TPM-backed host.\n" +
			"Real production fleets often run both while stronger hardware identity rolls out.",
		Action: func() error {
			if certs.FilesExist(agent2CertPath, agent2KeyPath, serverCaPath) {
				ctx.Log.Info("DevID credentials already exist, skipping provisioning.")
				return nil
			}

			ca, err := certs.GenerateCA("Demo DevID CA")
			if err != nil {
				return err
			}
			agent, err := certs.GenerateAgentCert(ca, "SPIRE Agent DevID")
			if err != nil {
				return err
			}
			if err := certs.WriteFile(serverCaPath, ca.CertPEM, ctx.Log); err != nil {
				return err
			}
			if err := certs.WriteFile(agent2CertPath, agent.CertPEM, ctx.Log); err != nil {
				return err
			}
			return certs.WritePrivateKey(agent2KeyPath, agent.KeyPEM, ctx.Log)
		},
		Observe: "Credentials ready for agent-2.\n" +
			"SPIRE server will trust agent-2 through devid-ca.pem during x509pop attestation.\n" +
			"Agent-1 will use a join token instead of a certificate.\n" +
			"Two attestation methods now live in one trust domain.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Building Dashboard Image",
		Explain: "Dashboard is small helper service for learning and ops.\n" +
			"It calls SPIRE server API and shows agents, entries, and bundle in one UI.\n" +
			"We build image now so later startup stays quick and repeatable.",
		Action: func() error {
			containerfile := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, ctx.RepoRoot, ctx.Log)
		},
		Observe: "Dashboard image ready.\n" +
			"Later step will start container from local image with no extra build wait.\n" +
			"UI will help verify agents, entries, and shared trust bundle.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Metrics Collectors",
		Explain: "Graphite plus StatsD and Prometheus start before SPIRE components.\n" +
			"Early start matters because first server and agent events emit useful bootstrap metrics.\n" +
			"Production teams use telemetry to spot failed attestation, slow issuance, and outages.\n" +
			"Running both shows old and new monitoring styles side by side.",
		Action: func() error {
			return ctx.Compose.Up("graphite-statsd", "prometheus")
		},
		Observe: "Metrics collectors running.\n" +
			"Graphite is at http://localhost:8080 and Prometheus is at http://localhost:9090.\n" +
			"Both are ready to receive SPIRE telemetry from first startup events.\n" +
			"Later you can compare what each backend captured.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server",
		Explain: "Server is production-style control plane for whole trust domain.\n" +
			"Config loads two NodeAttestor plugins: join_token and x509pop.\n" +
			"Telemetry is enabled so attestation and issuance flow into monitoring backends.\n" +
			"One server can accept mixed node identity sources without splitting trust domain.",
		Action: func() error {
			return ctx.Compose.Up("spire-server")
		},
		Observe: "SPIRE server running with dual attestation support.\n" +
			"It can attest agents with either join_token or x509pop.\n" +
			"Telemetry export is active while server initializes plugins and datastore.\n" +
			"Next step checks health before agents connect.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for SPIRE Server Health",
		Explain: "Health check waits for full server readiness, not just container start.\n" +
			"Ready means datastore opened, plugins loaded, and API serving requests.\n" +
			"Agents should not try attestation until control plane is healthy.",
		Action: func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "Server healthy.\n" +
			"Both NodeAttestor plugins are loaded and ready.\n" +
			"Control plane can now accept agent attestation and workload registration.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Agent-1 (Join Token)",
		Explain: "Join token is simple bootstrap path for node with no strong device identity.\n" +
			"Think cloud VM, ephemeral lab host, or early environment before hardware rollout.\n" +
			"Server mints one-time token, agent presents it, then long-term SPIFFE identity takes over.\n" +
			"After attestation, workload API on agent-1 will serve one workload pool.",
		Action: func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_1_JOIN_TOKEN", token)
			defer podman.UnsetEnv("SPIRE_AGENT_1_JOIN_TOKEN")

			podman.RemoveContainer("production-spire-agent-1", ctx.Log)
			return ctx.Compose.Up("spire-agent-1")
		},
		Observe: "Agent-1 starting with join token.\n" +
			"Server will record attestation type as join_token in agent list.\n" +
			"Use this as model for nodes that can bootstrap but lack hardware proof.\n" +
			"Workloads behind agent-1 will later inherit its trust path.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Agent-2 (x509pop)",
		Explain: "x509pop uses proof of possession for device certificate and private key.\n" +
			"Here DevID cert stands in for hardware-backed identity from TPM or secure element.\n" +
			"Server validates chain to trusted CA instead of checking shared secret token.\n" +
			"Many production bare-metal or appliance nodes use stronger path like this.",
		Action: func() error {
			return ctx.Compose.Up("spire-agent-2")
		},
		Observe: "Agent-2 starting with DevID certificate.\n" +
			"Server will record attestation type as x509pop in agent list.\n" +
			"No shared bootstrap secret is needed because trust comes from PKI chain.\n" +
			"Agent-2 now represents separate workload pool with stronger node proof.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Both Agents to Attest",
		Explain: "We wait for both node pools before starting workloads.\n" +
			"Each agent serves different workloads through its own Workload API socket.\n" +
			"If one agent fails attestation, only part of fleet can receive SVIDs.\n" +
			"Healthy result proves server accepts both bootstrap styles in one deployment.",
		Action: func() error {
			_, err := spirectl.WaitForAgents(ctx.Compose, "spire-server",
				[]string{"join_token", "x509pop"}, ctx.Log)
			return err
		},
		Observe: "Both agents attested.\n" +
			"Try: " + ctx.ComposeCmd() + " exec spire-server /opt/spire/bin/spire-server agent list.\n" +
			"You should see two agents with different attestation types and different SPIFFE IDs.\n" +
			"Each one is ready to serve its own workload pool.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Workload Containers",
		Explain: "Now app containers join, one behind each agent.\n" +
			"Each workload mounts only its local Workload API socket, not server API.\n" +
			"SPIFFE design keeps workload identity close to node agent that already attested.\n" +
			"Separate agents let teams isolate pools, zones, or hardware classes.",
		Action: func() error {
			return ctx.Compose.Up("workload-1", "workload-2")
		},
		Observe: "Workloads running.\n" +
			"Each workload watches its local agent socket for SVID delivery.\n" +
			"No workload talks to SPIRE server directly for day-to-day identity fetch.\n" +
			"Next step creates entries so agents know what each workload may become.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Registering Workloads",
		Explain: "Registration entries bind selectors and parent IDs to workload SPIFFE IDs.\n" +
			"Parent ID says which attested agent may vouch for workload.\n" +
			"workload-1 uses agent-1 SPIFFE ID, while workload-2 uses agent-2 SPIFFE ID.\n" +
			"Production systems use parent IDs to route identity through correct node pool.",
		Action: func() error {
			agent1ID, err := spirectl.GetAgentIDByType(ctx.Compose, "spire-server", "join_token", ctx.Log)
			if err != nil {
				return fmt.Errorf("no join_token agent found: %w", err)
			}
			agent2ID, err := spirectl.GetAgentIDByType(ctx.Compose, "spire-server", "x509pop", ctx.Log)
			if err != nil {
				return fmt.Errorf("no x509pop agent found: %w", err)
			}
			if err := spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/workload-1", agent1ID, "unix:uid:0", ctx.Log); err != nil {
				return err
			}
			return spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/workload-2", agent2ID, "unix:uid:0", ctx.Log)
		},
		Observe: "Two entries registered with different parent IDs.\n" +
			"Each workload gets its SVID from its own agent, not from other node pool.\n" +
			"Check dashboard Entries page to see both entries and parent relationships.\n" +
			"Same trust domain can still enforce clean workload-to-agent boundaries.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: "Dashboard gives one live view across agents, entries, and trust data.\n" +
			"In production, tools like this help operators inspect state without raw CLI only.\n" +
			"It is especially useful now because scenario has mixed attestors and workload pools.",
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8090/health")
			return nil
		},
		Observe: "Dashboard at http://localhost:8090.\n" +
			"Key things to explore: Agents page shows two agents with different types.\n" +
			"Entries page shows two workloads with different parent IDs.\n" +
			"Trust Bundle page shows shared trust domain used by whole deployment.",
	})
	if err != nil {
		return err
	}

	return nil
}

func done(ctx *scenario.Context) {
	ctx.Log.Step("Production scenario is ready!")
	ctx.Log.Info("Dashboard:    http://localhost:8090")
	ctx.Log.Info("Prometheus:   http://localhost:9090")
	ctx.Log.Info("Graphite:     http://localhost:8080")
	ctx.Log.Info("")
	ctx.Log.Info("This production-like setup demonstrates:")
	ctx.Log.Info("  - Multiple attestation methods (join_token + x509pop)")
	ctx.Log.Info("  - Multiple agents serving different workload pools")
	ctx.Log.Info("  - Full observability with Prometheus and Graphite")
	ctx.Log.Info("  - Centralized dashboard for SPIRE API monitoring")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-07-production down")
}
