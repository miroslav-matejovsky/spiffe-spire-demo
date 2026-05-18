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
	scenarioDir := filepath.Join(repoRoot, "scenarios", "05-svid-api")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down SVID API scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("SVID API scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// --- Step 1: Build Container Images ---
	err := runner.Run(
		"Building Container Images",
		"This scenario has three custom containers to build:\n"+
			"  - Dashboard: web UI for SPIRE server monitoring\n"+
			"  - svid-server: Go HTTPS server using SPIFFE mTLS\n"+
			"  - svid-client: Go HTTPS client using SPIFFE mTLS\n\n"+
			"Both Go services use the go-spiffe library to connect to the Workload API\n"+
			"and automatically fetch/rotate their X.509 SVIDs.",
		func() error {
			dashboardCf := filepath.Join(repoRoot, "dashboard", "Containerfile")
			serverCf := filepath.Join(scenarioDir, "server", "Containerfile")
			clientCf := filepath.Join(scenarioDir, "client", "Containerfile")

			if err := podman.Build("spiffe-spire-demo-dashboard:local", dashboardCf, repoRoot, log); err != nil {
				return err
			}
			if err := podman.Build("spiffe-spire-demo-svid-server:local", serverCf, repoRoot, log); err != nil {
				return err
			}
			return podman.Build("spiffe-spire-demo-svid-client:local", clientCf, repoRoot, log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 2: Start SPIRE Server ---
	err = runner.Run(
		"Starting SPIRE Server",
		"Starting the trust domain authority for mirmat.org.",
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
		"Polling until the server API is ready.",
		func() error {
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Token + Agent ---
	err = runner.Run(
		"Starting SPIRE Agent",
		"Generate a join token and start the agent. The agent shares a Unix socket\n"+
			"volume with the svid-server and svid-client containers. This socket is\n"+
			"the Workload API endpoint where services fetch their SVIDs.",
		func() error {
			token, err := spirectl.GenerateToken(compose, "spire-server", "spiffe://mirmat.org/myagent", log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_JOIN_TOKEN", token)
			return compose.Up("spire-agent")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 5: Attestation ---
	err = runner.Run(
		"Waiting for Agent Attestation",
		"Agent attests using its join token.",
		func() error {
			_, err := spirectl.WaitForAgent(compose, "spire-server", "join_token", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 6: Register Workloads ---
	err = runner.Run(
		"Registering Workloads",
		"Workload registration maps a process selector to a SPIFFE ID:\n\n"+
			"  svid-server (UID 10001) -> spiffe://mirmat.org/svid-server\n"+
			"  svid-client (UID 10002) -> spiffe://mirmat.org/svid-client\n\n"+
			"The SPIRE agent watches processes on its node. When a process with UID\n"+
			"10001 connects to the Workload API, it receives the svid-server identity.\n"+
			"This is how SPIFFE delivers identity without application changes.",
		func() error {
			agentID, err := spirectl.GetAgentID(compose, "spire-server", log)
			if err != nil {
				return err
			}
			if err := spirectl.CreateEntry(compose, "spire-server",
				"spiffe://mirmat.org/svid-server", agentID, "unix:uid:10001", log); err != nil {
				return err
			}
			return spirectl.CreateEntry(compose, "spire-server",
				"spiffe://mirmat.org/svid-client", agentID, "unix:uid:10002", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 7: Start Go Services ---
	err = runner.Run(
		"Starting mTLS Services",
		"Starting svid-server and svid-client. Both connect to the Workload API\n"+
			"socket, fetch their SVIDs, and establish mTLS communication:\n\n"+
			"  1. svid-server listens on :8443 with its SVID as the server cert\n"+
			"  2. svid-client calls the server, presenting its own SVID\n"+
			"  3. Both verify the peer's identity against the trust bundle\n\n"+
			"The client makes 5 requests and logs the responses.",
		func() error {
			if err := compose.UpNoBuild("svid-server"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return compose.UpNoBuild("svid-client")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 8: Start Dashboard ---
	err = runner.Run(
		"Starting Dashboard",
		"Starting the dashboard for monitoring.",
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
	log.Step("SVID API scenario is ready!")
	log.Info("Dashboard:    http://localhost:8080")
	log.Info("Server logs:  podman-compose logs -f svid-server")
	log.Info("Client logs:  podman-compose logs -f svid-client")
	log.Info("")
	log.Info("What is happening:")
	log.Info("  - svid-server has SPIFFE ID: spiffe://mirmat.org/svid-server")
	log.Info("  - svid-client has SPIFFE ID: spiffe://mirmat.org/svid-client")
	log.Info("  - They communicate over mTLS using auto-rotated X.509 SVIDs")
	log.Info("  - No certificates were manually configured in the applications")
	log.Info("")
	log.Info("Tear down with: scenario-05-svid-api --down")

	// Cleanup env
	podman.UnsetEnv("SPIRE_AGENT_JOIN_TOKEN")
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
