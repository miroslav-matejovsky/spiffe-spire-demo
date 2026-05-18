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
	scenarioDir := filepath.Join(repoRoot, "scenarios", "04-tpm")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down TPM scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("TPM scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// Certificate paths
	serverCaPath := filepath.Join(scenarioDir, "spire", "server", "devid-ca.pem")
	agentCertPath := filepath.Join(scenarioDir, "spire", "agent", "devid-cert.pem")
	agentKeyPath := filepath.Join(scenarioDir, "spire", "agent", "devid-key.pem")

	// --- Step 1: Provision Certificates ---
	err := runner.Run(
		"Provisioning x509pop Certificates",
		"In a real environment, a TPM holds a DevID certificate provisioned during\n"+
			"manufacturing. For this demo, we generate equivalent certificates:\n\n"+
			"  - A self-signed CA certificate (trusted by the SPIRE server)\n"+
			"  - An agent certificate signed by that CA (proves agent identity)\n"+
			"  - The agent's private key\n\n"+
			"The SPIRE server is configured to trust the CA, so any agent presenting\n"+
			"a certificate signed by this CA will be attested via x509pop.",
		func() error {
			if certs.FilesExist(serverCaPath, agentCertPath, agentKeyPath) {
				log.Info("Certificates already exist, skipping provisioning.")
				return nil
			}

			log.Info("Generating CA certificate...")
			ca, err := certs.GenerateCA("Demo DevID CA")
			if err != nil {
				return err
			}

			log.Info("Generating agent certificate...")
			agent, err := certs.GenerateAgentCert(ca, "SPIRE Agent DevID")
			if err != nil {
				return err
			}

			// Write CA cert to server directory
			if err := certs.WriteFile(serverCaPath, ca.CertPEM, log); err != nil {
				return err
			}
			// Write agent cert and key
			if err := certs.WriteFile(agentCertPath, agent.CertPEM, log); err != nil {
				return err
			}
			if err := certs.WriteFile(agentKeyPath, agent.KeyPEM, log); err != nil {
				return err
			}

			log.Ok("Certificates provisioned.")
			return nil
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 2: Build Dashboard ---
	err = runner.Run(
		"Building Dashboard Image",
		"Building the dashboard container for monitoring SPIRE state.",
		func() error {
			containerfile := filepath.Join(repoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, repoRoot, log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 3: Start SPIRE Services ---
	err = runner.Run(
		"Starting SPIRE Server and Agent",
		"Both the server and agent start together. Unlike scenario 01 where the\n"+
			"agent needed a join token, here the agent automatically attests using\n"+
			"its x509pop certificate. The server validates the cert chain against\n"+
			"the trusted CA we provisioned earlier.\n\n"+
			"No shared secret (token) is needed - the trust is established through\n"+
			"the PKI hierarchy, just like real TPM-based attestation.",
		func() error {
			return compose.Up("spire-server", "spire-agent")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Wait for containers ---
	err = runner.Run(
		"Waiting for Containers",
		"Waiting for both SPIRE server and agent containers to be running.",
		func() error {
			time.Sleep(5 * time.Second)
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 5: Wait for Attestation ---
	err = runner.Run(
		"Waiting for x509pop Attestation",
		"The agent presents its certificate to the server. The server:\n"+
			"  1. Validates the certificate signature against the trusted CA\n"+
			"  2. Verifies the agent possesses the corresponding private key\n"+
			"  3. Registers the agent with attestation type 'x509pop'\n\n"+
			"This is more secure than join tokens because the private key never\n"+
			"leaves the agent (in real TPM scenarios, it never leaves the hardware).",
		func() error {
			_, err := spirectl.WaitForAgent(compose, "spire-server", "x509pop", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 6: Start Dashboard ---
	err = runner.Run(
		"Starting Dashboard",
		"Starting the dashboard to view the attested agent.",
		func() error {
			if err := compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			return waitForDashboard("http://127.0.0.1:8080/health", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Done ---
	log.Step("TPM scenario is ready!")
	log.Info("Dashboard: http://localhost:8080")
	log.Info("")
	log.Info("Key difference from scenario 01:")
	log.Info("  - No join token was needed")
	log.Info("  - Agent proved identity via certificate (x509pop)")
	log.Info("  - In production, this certificate lives in a TPM chip")
	log.Info("")
	log.Info("Tear down with: scenario-04-tpm --down")
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
