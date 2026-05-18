package main

import (
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/certs"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
)

func main() {
	stepMode := flag.Bool("step", false, "pause between steps for interactive learning")
	verbose := flag.Bool("verbose", false, "show detailed debug output")
	down := flag.Bool("down", false, "tear down the scenario")
	flag.Parse()

	log := logging.New(*verbose)
	repoRoot := findRepoRoot()
	scenarioDir := filepath.Join(repoRoot, "scenarios", "06-metrics")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down metrics scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("Metrics scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// Certificate paths for x509pop attestation
	serverCaPath := filepath.Join(scenarioDir, "spire", "server", "agent-cacert.pem")
	agentCertPath := filepath.Join(scenarioDir, "spire", "agent", "agent.crt.pem")
	agentKeyPath := filepath.Join(scenarioDir, "spire", "agent", "agent.key.pem")

	// --- Step 1: Provision Certificates ---
	err := runner.Run(
		"Provisioning Agent Credentials",
		"Like scenario 04, this scenario uses x509pop attestation.\n"+
			"We generate a CA and agent certificate for the SPIRE agent.",
		func() error {
			if certs.FilesExist(serverCaPath, agentCertPath, agentKeyPath) {
				log.Info("Certificates already exist, skipping provisioning.")
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
			if err := certs.WriteFile(serverCaPath, ca.CertPEM, log); err != nil {
				return err
			}
			if err := certs.WriteFile(agentCertPath, agent.CertPEM, log); err != nil {
				return err
			}
			return certs.WriteFile(agentKeyPath, agent.KeyPEM, log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 2: Build Dashboard ---
	err = runner.Run(
		"Building Dashboard Image",
		"Building the dashboard container.",
		func() error {
			containerfile := filepath.Join(repoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, repoRoot, log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 3: Start Metrics Collectors + SPIRE ---
	err = runner.Run(
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
			return compose.Up("graphite-statsd", "prometheus", "spire-server", "spire-agent")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Wait for services ---
	err = runner.Run(
		"Waiting for Services",
		"Waiting for SPIRE server and metrics collectors to initialize.",
		func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 5: Start Dashboard ---
	err = runner.Run(
		"Starting Dashboard",
		"Starting the SPIRE dashboard on port 8090.",
		func() error {
			if err := compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			return waitForDashboard("http://127.0.0.1:8090/health", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Done ---
	log.Step("Metrics scenario is ready!")
	log.Info("Dashboard:    http://localhost:8090")
	log.Info("Prometheus:   http://localhost:9090")
	log.Info("Graphite:     http://localhost:8080")
	log.Info("")
	log.Info("Explore metrics:")
	log.Info("  - Prometheus: query 'spire_server' prefix for SPIRE metrics")
	log.Info("  - Graphite: browse the 'stats' tree for StatsD counters")
	log.Info("  - Dashboard: see agent and entry counts from SPIRE API")
	log.Info("")
	log.Info("Tear down with: scenario-06-metrics --down")
}

func waitForDashboard(url string, log *logging.Logger) error {
	log.Info("Waiting for dashboard to become ready...")
	client := &http.Client{Timeout: 2 * time.Second}
	for attempt := 1; attempt <= 30; attempt++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Ok("Dashboard is ready.")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		log.Detailf("attempt %d/30: waiting for dashboard...", attempt)
		time.Sleep(2 * time.Second)
	}
	log.Warn("Dashboard did not respond in time.")
	return nil
}

func findRepoRoot() string {
	cwd, _ := os.Getwd()
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd
}
