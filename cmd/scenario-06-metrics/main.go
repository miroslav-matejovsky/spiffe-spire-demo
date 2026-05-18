package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "Metrics",
		DirName: "06-metrics",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	serverCaPath := filepath.Join(ctx.ScenarioDir, "spire", "server", "agent-cacert.pem")
	agentCertPath := filepath.Join(ctx.ScenarioDir, "spire", "agent", "agent.crt.pem")
	agentKeyPath := filepath.Join(ctx.ScenarioDir, "spire", "agent", "agent.key.pem")

	err := ctx.Runner.Run(
		"Provisioning Agent Credentials",
		"Like scenario 04, this scenario uses x509pop attestation.\n"+
			"We generate a CA and agent certificate for the SPIRE agent.",
		func() error {
			if certs.FilesExist(serverCaPath, agentCertPath, agentKeyPath) {
				ctx.Log.Info("Certificates already exist, skipping provisioning.")
				return nil
			}

			ca, err := certs.GenerateCA("Demo x509pop CA")
			if err != nil {
				return err
			}
			agent, err := certs.GenerateAgentCert(ca, "SPIRE Agent x509pop")
			if err != nil {
				return err
			}
			if err := certs.WriteFile(serverCaPath, ca.CertPEM, ctx.Log); err != nil {
				return err
			}
			if err := certs.WriteFile(agentCertPath, agent.CertPEM, ctx.Log); err != nil {
				return err
			}
			return certs.WritePrivateKey(agentKeyPath, agent.KeyPEM, ctx.Log)
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
		"Starting Core Services",
		"Starting the metrics collection stack alongside SPIRE:\n\n"+
			"  - Graphite + StatsD: receives SPIRE's StatsD-format metrics\n"+
			"  - Prometheus: scrapes SPIRE server's /metrics endpoint\n"+
			"  - SPIRE server: configured with telemetry block for StatsD output\n"+
			"  - SPIRE agent: uses x509pop attestation\n\n"+
			"SPIRE's server.conf has a telemetry section that sends counters and\n"+
			"gauges to the StatsD receiver on each operation (token generation,\n"+
			"SVID signing, etc.).",
		func() error {
			return ctx.Compose.Up("graphite-statsd", "prometheus", "spire-server", "spire-agent")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for Services",
		"Waiting for SPIRE server and metrics collectors to initialize.",
		func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
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
	ctx.Log.Step("Metrics scenario is ready!")
	ctx.Log.Info("Dashboard:    http://localhost:8090")
	ctx.Log.Info("Prometheus:   http://localhost:9090")
	ctx.Log.Info("Graphite:     http://localhost:8080")
	ctx.Log.Info("")
	ctx.Log.Info("Explore metrics:")
	ctx.Log.Info("  - Prometheus: query 'spire_server' prefix for SPIRE metrics")
	ctx.Log.Info("  - Graphite: browse the 'stats' tree for StatsD counters")
	ctx.Log.Info("  - Dashboard: see agent and entry counts from SPIRE API")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-06-metrics down")
}
