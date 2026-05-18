package main

import (
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	scenarioDir := filepath.Join(repoRoot, "scenarios", "02-dashboard")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down dashboard scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("Dashboard scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// --- Step 1: Build Dashboard Image ---
	err := runner.Run(
		"Building Dashboard Image",
		"The dashboard is a Go web application that connects to the SPIRE server's\n"+
			"gRPC API. It displays agents, registered entries, and trust bundle info.\n\n"+
			"We build it as a container image using a multi-stage Containerfile.",
		func() error {
			containerfile := filepath.Join(repoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, repoRoot, log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 2: Start SPIRE Server ---
	err = runner.Run(
		"Starting SPIRE Server",
		"Same as scenario 01 - the SPIRE server is the trust domain authority.",
		func() error {
			return compose.Up("spire-server")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 3: Wait for SPIRE Server ---
	err = runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling the server healthcheck until it is ready to accept API calls.",
		func() error {
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Generate Token and Start Agent ---
	err = runner.Run(
		"Starting SPIRE Agent with Join Token",
		"We generate a join token and start the agent in one step.\n"+
			"The agent will attest to the server automatically on startup.",
		func() error {
			token, err := spirectl.GenerateToken(compose, "spire-server", "spiffe://mirmat.org/myagent", log)
			if err != nil {
				return err
			}
			agentContainer := "dashboard-spire-agent"
			podman.RemoveContainer(agentContainer, log)
			return compose.RunDetached(agentContainer, "spire-agent",
				"-config", "/opt/spire/conf/agent/agent.conf",
				"-joinToken", token,
			)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 5: Wait for Attestation ---
	err = runner.Run(
		"Waiting for Agent Attestation",
		"The agent presents its join token to the server for attestation.",
		func() error {
			_, err := spirectl.WaitForAgent(compose, "spire-server", "join_token", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 6: Start Dashboard ---
	err = runner.Run(
		"Starting Dashboard",
		"The dashboard container connects to the SPIRE server's admin API socket.\n"+
			"It is mounted via a shared volume so the dashboard can query agents,\n"+
			"entries, and bundles in real-time.\n\n"+
			"Dashboard URL: http://localhost:8080",
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
	log.Step("Dashboard scenario is ready!")
	log.Info("Dashboard: http://localhost:8080")
	log.Info("")
	log.Info("What you can explore:")
	log.Info("  - Overview page shows server health, agent count, entry count")
	log.Info("  - Agents page shows the attested agent and its SPIFFE ID")
	log.Info("  - Trust Bundle page shows the X.509 authorities for the domain")
	log.Info("")
	log.Info("Tear down with: scenario-02-dashboard --down")
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
	log.Warn("Dashboard did not respond in time -- it may still be starting.")
	return nil
}

func findRepoRoot() string {
	cwd, _ := os.Getwd()
	// Walk up looking for go.mod
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
	// Fallback to cwd
	return cwd
}
