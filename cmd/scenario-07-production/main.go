package main

import (
	"fmt"
	"path/filepath"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
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

	err := ctx.Runner.Run(
		"Provisioning DevID Credentials for Agent-2",
		"Agent-2 uses x509pop attestation with a DevID certificate, simulating\n"+
			"a hardware TPM. We generate:\n"+
			"  - A CA certificate (trusted by the SPIRE server)\n"+
			"  - An agent certificate + key (used by agent-2 for attestation)\n\n"+
			"Agent-1 uses a simpler join_token method for comparison.",
		func() error {
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
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Building Dashboard Image",
		"Building the dashboard container.",
		func() error {
			containerfile := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, ctx.RepoRoot, ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Metrics Collectors",
		"Starting Graphite (StatsD receiver) and Prometheus for observability.\n"+
			"These start first so they are ready to receive metrics when SPIRE starts.",
		func() error {
			return ctx.Compose.Up("graphite-statsd", "prometheus")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Server",
		"The production SPIRE server is configured with:\n"+
			"  - Telemetry output to StatsD (Graphite)\n"+
			"  - Two NodeAttestor plugins: join_token and x509pop\n"+
			"  - The DevID CA loaded for x509pop verification",
		func() error {
			return ctx.Compose.Up("spire-server")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling server healthcheck.",
		func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Agent-1 (Join Token)",
		"Agent-1 uses the simplest attestation method: a one-time join token.\n"+
			"Good for initial bootstrapping or development environments.",
		func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_1_JOIN_TOKEN", token)
			defer podman.UnsetEnv("SPIRE_AGENT_1_JOIN_TOKEN")

			podman.RemoveContainer("production-spire-agent-1", ctx.Log)
			return ctx.Compose.Up("spire-agent-1")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Agent-2 (x509pop)",
		"Agent-2 uses certificate-based attestation (x509pop).\n"+
			"It presents its DevID certificate to the server for verification.\n"+
			"No shared secret needed - trust flows from the PKI hierarchy.",
		func() error {
			return ctx.Compose.Up("spire-agent-2")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for Both Agents to Attest",
		"Both agents must attest successfully:\n"+
			"  - Agent-1: presents join_token to server\n"+
			"  - Agent-2: presents x509pop certificate to server\n\n"+
			"The server validates each using the appropriate NodeAttestor plugin.",
		func() error {
			_, err := spirectl.WaitForAgents(ctx.Compose, "spire-server",
				[]string{"join_token", "x509pop"}, ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Workload Containers",
		"Starting workload-1 and workload-2. These represent application services\n"+
			"that will receive SPIFFE identities from their respective agents.",
		func() error {
			return ctx.Compose.Up("workload-1", "workload-2")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Registering Workloads",
		"Creating workload registration entries that map selectors to SPIFFE IDs.\n"+
			"In production, this would be done by a CI/CD pipeline or registration API.",
		func() error {
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
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Dashboard",
		"Starting the SPIRE dashboard on port 8090.",
		func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8090/health")
			return nil
		},
	)
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
