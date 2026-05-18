package main

import (
	"flag"
	"fmt"
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
	scenarioDir := filepath.Join(repoRoot, "scenarios", "07-production")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down production scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("Production scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// DevID certificate paths for agent-2
	agent2CertPath := filepath.Join(scenarioDir, "spire", "agent-2", "devid-cert.pem")
	agent2KeyPath := filepath.Join(scenarioDir, "spire", "agent-2", "devid-key.pem")
	serverCaPath := filepath.Join(scenarioDir, "spire", "server", "devid-ca.pem")

	// --- Step 1: Provision DevID Credentials ---
	err := runner.Run(
		"Provisioning DevID Credentials for Agent-2",
		"Agent-2 uses x509pop attestation with a DevID certificate, simulating\n"+
			"a hardware TPM. We generate:\n"+
			"  - A CA certificate (trusted by the SPIRE server)\n"+
			"  - An agent certificate + key (used by agent-2 for attestation)\n\n"+
			"Agent-1 uses a simpler join_token method for comparison.",
		func() error {
			if certs.FilesExist(agent2CertPath, agent2KeyPath, serverCaPath) {
				log.Info("DevID credentials already exist, skipping provisioning.")
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
			if err := certs.WriteFile(serverCaPath, ca.CertPEM, log); err != nil {
				return err
			}
			if err := certs.WriteFile(agent2CertPath, agent.CertPEM, log); err != nil {
				return err
			}
			return certs.WriteFile(agent2KeyPath, agent.KeyPEM, log)
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

	// --- Step 3: Start Metrics Collectors ---
	err = runner.Run(
		"Starting Metrics Collectors",
		"Starting Graphite (StatsD receiver) and Prometheus for observability.\n"+
			"These start first so they are ready to receive metrics when SPIRE starts.",
		func() error {
			return compose.Up("graphite-statsd", "prometheus")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Start SPIRE Server ---
	err = runner.Run(
		"Starting SPIRE Server",
		"The production SPIRE server is configured with:\n"+
			"  - Telemetry output to StatsD (Graphite)\n"+
			"  - Two NodeAttestor plugins: join_token and x509pop\n"+
			"  - The DevID CA loaded for x509pop verification",
		func() error {
			return compose.Up("spire-server")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 5: Wait for Server ---
	err = runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling server healthcheck.",
		func() error {
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 6: Start Agent-1 (join_token) ---
	err = runner.Run(
		"Starting Agent-1 (Join Token)",
		"Agent-1 uses the simplest attestation method: a one-time join token.\n"+
			"Good for initial bootstrapping or development environments.",
		func() error {
			token, err := spirectl.GenerateToken(compose, "spire-server", "spiffe://mirmat.org/myagent", log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_1_JOIN_TOKEN", token)

			podman.RemoveContainer("production-spire-agent-1", log)
			return compose.Up("spire-agent-1")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 7: Start Agent-2 (x509pop) ---
	err = runner.Run(
		"Starting Agent-2 (x509pop)",
		"Agent-2 uses certificate-based attestation (x509pop).\n"+
			"It presents its DevID certificate to the server for verification.\n"+
			"No shared secret needed - trust flows from the PKI hierarchy.",
		func() error {
			return compose.Up("spire-agent-2")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 8: Wait for Both Agents ---
	err = runner.Run(
		"Waiting for Both Agents to Attest",
		"Both agents must attest successfully:\n"+
			"  - Agent-1: presents join_token to server\n"+
			"  - Agent-2: presents x509pop certificate to server\n\n"+
			"The server validates each using the appropriate NodeAttestor plugin.",
		func() error {
			_, err := spirectl.WaitForAgents(compose, "spire-server",
				[]string{"join_token", "x509pop"}, log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 9: Start Workload Containers ---
	err = runner.Run(
		"Starting Workload Containers",
		"Starting workload-1 and workload-2. These represent application services\n"+
			"that will receive SPIFFE identities from their respective agents.",
		func() error {
			return compose.Up("workload-1", "workload-2")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 10: Register Workloads ---
	err = runner.Run(
		"Registering Workloads",
		"Creating workload registration entries that map selectors to SPIFFE IDs.\n"+
			"In production, this would be done by a CI/CD pipeline or registration API.",
		func() error {
			agentID, err := spirectl.GetAgentID(compose, "spire-server", log)
			if err != nil {
				return fmt.Errorf("no agent found: %w", err)
			}
			if err := spirectl.CreateEntry(compose, "spire-server",
				"spiffe://mirmat.org/workload-1", agentID, "unix:uid:0", log); err != nil {
				return err
			}
			return spirectl.CreateEntry(compose, "spire-server",
				"spiffe://mirmat.org/workload-2", agentID, "unix:uid:0", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 11: Start Dashboard ---
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
	log.Step("Production scenario is ready!")
	log.Info("Dashboard:    http://localhost:8090")
	log.Info("Prometheus:   http://localhost:9090")
	log.Info("Graphite:     http://localhost:8080")
	log.Info("")
	log.Info("This production-like setup demonstrates:")
	log.Info("  - Multiple attestation methods (join_token + x509pop)")
	log.Info("  - Multiple agents serving different workload pools")
	log.Info("  - Full observability with Prometheus and Graphite")
	log.Info("  - Centralized dashboard for SPIRE API monitoring")
	log.Info("")
	log.Info("Tear down with: scenario-07-production --down")

	// Cleanup env
	podman.UnsetEnv("SPIRE_AGENT_1_JOIN_TOKEN")
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
