package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
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

	err := ctx.Runner.RunStep(step.Step{
		Name: "Provisioning Agent Credentials",
		Explain: "SPIRE agent in scenario joins with x509pop attestation.\n" +
			"Server trusts CA file in spire/server/agent-cacert.pem, then checks agent leaf cert.\n" +
			"Like scenario 04, we mint demo CA, agent certificate, and private key before startup.\n" +
			"Without these files, agent cannot prove key possession during node attestation.",
		Sources: []step.Source{
			ctx.Src("spire/server/server.conf", 18, "NodeAttestor x509pop with CA bundle"),
			ctx.Src("spire/agent/agent.conf", 12, "x509pop with cert and key paths"),
			ctx.RepoSrc("internal/certs/certs.go", 0, "certificate generation helpers"),
		},
		Action: func() error {
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
		Observe: "Certificates ready for x509pop attestation.\n" +
			"Server now has trusted CA material, and agent has leaf cert plus private key.\n" +
			"When agent container starts, it can prove key ownership and receive SPIFFE identity.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Building Dashboard Image",
		Explain: "Scenario keeps SPIRE dashboard for API view beside telemetry tools.\n" +
			"We build local image now so compose can start dashboard without extra delay later.\n" +
			"Dashboard complements metrics by showing entries, agents, and bundles from SPIRE API.",
		Action: func() error {
			return podman.BuildDashboard(ctx.RepoRoot, ctx.Log)
		},
		Observe: "Dashboard image ready.\n" +
			"UI container can now join scenario without rebuild work.\n" +
			"Later compare dashboard API state with Prometheus queries and Graphite charts.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Core Services",
		Explain: "Now stack starts two telemetry paths from telemetry {} blocks in SPIRE config.\n" +
			"Prometheus is pull based: SPIRE exposes HTTP metrics on spire-server:8088 and spire-agent:8089.\n" +
			"Prometheus scrapes both targets at metrics path \"/\", not usual \"/metrics\".\n" +
			"StatsD is push based: SPIRE server sends counters and gauges to graphite-statsd:8125.\n" +
			"Graphite stores pushed series, while Prometheus stores scraped time series for queries.\n" +
			"Same control plane now teaches both push and pull telemetry models.",
		Sources: []step.Source{
			ctx.Src("spire/server/server.conf", 28, "server telemetry: Prometheus + StatsD"),
			ctx.Src("spire/agent/agent.conf", 29, "agent telemetry: Prometheus + StatsD"),
			ctx.Src("prometheus/prometheus.yml", 0, "Prometheus scrape configuration"),
			ctx.Src("compose.yml", 4, "graphite-statsd service"),
			ctx.Src("compose.yml", 12, "prometheus service"),
		},
		Action: func() error {
			return ctx.Compose.Up("graphite-statsd", "prometheus", "spire-server", "spire-agent")
		},
		Observe: "All services starting.\n" +
			"SPIRE server now emits metrics to both Prometheus by exposing :8088 for pull scraping and StatsD by pushing to :8125.\n" +
			"SPIRE agent exposes its own Prometheus endpoint on :8089 for scrape by Prometheus.\n" +
			"Graphite UI lives at http://localhost:8080 and Prometheus UI lives at http://localhost:9090.\n" +
			"Open Prometheus and search for metrics with \"spire_server\" prefix.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Services",
		Explain: "Containers need few seconds before health and telemetry settle.\n" +
			"We wait for SPIRE server healthcheck and give scrapers time to grab first samples.\n" +
			"Healthy control plane means metrics endpoints and StatsD pipeline should soon show data.",
		Action: func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "Server healthy. Metrics should be flowing.\n" +
			"Check Prometheus targets page at http://localhost:9090/targets and verify scrape endpoints are UP.\n" +
			"If targets are UP, Prometheus can scrape SPIRE at \"/\" and store fresh samples.\n" +
			"Graphite should also begin receiving pushed counters from StatsD.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: "Last step starts dashboard on port 8090.\n" +
			"Dashboard reads SPIRE API state, not telemetry stream, so it shows different observability layer.\n" +
			"Use it beside Prometheus and Graphite to compare objects, health, and metrics.",
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8090/health")
			return nil
		},
		Observe: "Dashboard ready at http://localhost:8090.\n" +
			"Use dashboard alongside Prometheus and Graphite to see SPIRE state from both API and metrics perspectives.\n" +
			"Same event can appear as agents or entries in dashboard and as counters or gauges in telemetry.",
	})
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
