package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	scenarioDir := filepath.Join(repoRoot, "scenarios", "03-workload")
	compose := podman.NewCompose(scenarioDir, log)

	// Handle subcommands
	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "register":
			runRegister(compose, log, *stepMode)
			return
		case "fetch":
			runFetch(compose, log)
			return
		}
	}

	if *down {
		log.Step("Tearing down workload scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("Workload scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// --- Step 1: Build Dashboard ---
	err := runner.Run(
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

	// --- Step 2: Start SPIRE Server ---
	err = runner.Run(
		"Starting SPIRE Server",
		"Starting the SPIRE server - the trust domain authority for mirmat.org.",
		func() error {
			return compose.Up("spire-server")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 3: Healthcheck ---
	err = runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling server healthcheck until ready.",
		func() error {
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Token + Agent ---
	err = runner.Run(
		"Starting SPIRE Agent with Join Token",
		"Generate a join token, then start the agent. The agent exposes the\n"+
			"Workload API socket that workloads connect to for identity.",
		func() error {
			token, err := spirectl.GenerateToken(compose, "spire-server", "spiffe://mirmat.org/myagent", log)
			if err != nil {
				return err
			}
			agentContainer := "workload-spire-agent"
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

	// --- Step 5: Attestation ---
	err = runner.Run(
		"Waiting for Agent Attestation",
		"The agent attests to the server using the join token.",
		func() error {
			_, err := spirectl.WaitForAgent(compose, "spire-server", "join_token", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 6: Start Workload Container ---
	err = runner.Run(
		"Starting Workload Container",
		"The workload container shares the Workload API socket with the SPIRE agent\n"+
			"via a named volume. Once a workload entry is registered for this container,\n"+
			"it will automatically receive an X.509 SVID.",
		func() error {
			if err := compose.UpNoBuild("workload"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return nil
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 7: Start Dashboard ---
	err = runner.Run(
		"Starting Dashboard",
		"Starting the web dashboard for monitoring.",
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
	log.Step("Workload scenario is ready!")
	log.Info("Dashboard: http://localhost:8080")
	log.Info("")
	log.Info("Next steps:")
	log.Info("  1. Register a workload: scenario-03-workload register")
	log.Info("  2. Fetch the SVID:      scenario-03-workload fetch")
	log.Info("")
	log.Info("Tear down with: scenario-03-workload --down")
}

// runRegister registers a workload entry with the SPIRE server.
func runRegister(compose *podman.Compose, log *logging.Logger, stepMode bool) {
	runner := step.New(log, stepMode)

	err := runner.Run(
		"Registering Workload",
		"A workload registration entry tells SPIRE: 'any process matching this\n"+
			"selector should receive this SPIFFE ID'. We use unix:uid:0 (root) as\n"+
			"the selector, meaning any root process on the agent node gets the ID.\n\n"+
			"SPIFFE ID: spiffe://mirmat.org/myworkload\n"+
			"Selector:  unix:uid:0 (root user inside the container)",
		func() error {
			agentID, err := spirectl.GetAgentID(compose, "spire-server", log)
			if err != nil {
				return fmt.Errorf("no attested agent found - run scenario startup first: %w", err)
			}
			return spirectl.CreateEntry(compose, "spire-server",
				"spiffe://mirmat.org/myworkload", agentID, "unix:uid:0", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	log.Info("")
	log.Info("The workload container will now automatically receive its SVID.")
	log.Info("Run 'scenario-03-workload fetch' to see the issued SVID.")
}

// runFetch displays the SVID fetched by the workload container.
func runFetch(compose *podman.Compose, log *logging.Logger) {
	log.Step("Fetching workload SVID...")
	log.Info("Reading logs from the workload container to find SVID details...")

	for attempt := 1; attempt <= 30; attempt++ {
		output, err := compose.Exec("workload", "cat", "/tmp/svid-output.txt")
		if err == nil && strings.Contains(output, "Received 1 svid") {
			// Filter and display relevant lines
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "Received") ||
					strings.Contains(line, "SPIFFE ID") ||
					strings.Contains(line, "SVID Valid") ||
					strings.Contains(line, "CA #") {
					fmt.Println(line)
				}
			}
			log.Ok("SVID fetched successfully!")
			log.Info("The output above shows the X.509-SVID issued to the workload.")
			return
		}
		log.Detailf("attempt %d/30: waiting for SVID...", attempt)
		time.Sleep(2 * time.Second)
	}

	log.Error("Workload has not received an SVID yet.")
	log.Info("Make sure 'scenario-03-workload register' completed successfully.")
	os.Exit(1)
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
