package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

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
	scenarioDir := findScenarioDir("scenarios/01-simple")
	compose := podman.NewCompose(scenarioDir, log)

	if *down {
		log.Step("Tearing down simple scenario...")
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok("Simple scenario stopped.")
		return
	}

	runner := step.New(log, *stepMode)

	// --- Step 1: Start SPIRE Server ---
	err := runner.Run(
		"Starting SPIRE Server",
		"The SPIRE Server is the central authority in a SPIFFE trust domain.\n"+
			"It manages identity registration, signs X.509 SVIDs and JWT SVIDs,\n"+
			"and maintains the trust bundle that all workloads use for verification.\n\n"+
			"We start it as a container using podman-compose.",
		func() error {
			return compose.Up("spire-server")
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 2: Wait for SPIRE Server healthcheck ---
	err = runner.Run(
		"Waiting for SPIRE Server Health",
		"The SPIRE server needs a moment to initialize its datastore, load plugins,\n"+
			"and start listening on its API socket. We poll its healthcheck endpoint\n"+
			"until it reports ready.",
		func() error {
			return spirectl.Healthcheck(compose, "spire-server", log)
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 3: Generate Join Token ---
	var token string
	err = runner.Run(
		"Generating Join Token",
		"A join token is a one-time-use secret that allows a SPIRE agent to prove\n"+
			"its identity to the server during initial attestation. The server generates\n"+
			"the token and associates it with a SPIFFE ID for the agent.\n\n"+
			"Join tokens are the simplest attestation method. In production, you would\n"+
			"use hardware-based attestation (TPM, cloud instance identity, etc.).",
		func() error {
			var err error
			token, err = spirectl.GenerateToken(compose, "spire-server", "spiffe://mirmat.org/myagent", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Step 4: Start SPIRE Agent ---
	agentContainer := "simple-spire-agent"
	err = runner.Run(
		"Starting SPIRE Agent",
		"The SPIRE Agent runs on each node (or in each container) that hosts workloads.\n"+
			"It connects to the SPIRE Server using the join token for initial attestation.\n"+
			"Once attested, the agent can request SVIDs on behalf of its workloads.\n\n"+
			"The agent exposes the Workload API - a local Unix socket that workloads\n"+
			"connect to for fetching their identities.",
		func() error {
			// Remove leftover container from previous runs
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

	// --- Step 5: Wait for Agent Attestation ---
	var agentList string
	err = runner.Run(
		"Waiting for Agent Attestation",
		"After the agent starts, it presents the join token to the server.\n"+
			"The server validates the token (single-use), creates an agent record,\n"+
			"and issues the agent its own SVID. This process is called attestation.\n\n"+
			"We poll the server's agent list until we see our agent registered.",
		func() error {
			var err error
			agentList, err = spirectl.WaitForAgent(compose, "spire-server", "join_token", log)
			return err
		},
	)
	if err != nil {
		os.Exit(1)
	}

	// --- Done ---
	log.Step("Simple scenario is ready!")
	log.Info("Registered agents:")
	fmt.Println(agentList)
	log.Info(fmt.Sprintf("Server: %s", "simple-spire-server"))
	log.Info(fmt.Sprintf("Agent:  %s", agentContainer))
	log.Info("")
	log.Info("What just happened:")
	log.Info("  1. SPIRE server started and initialized the trust domain 'mirmat.org'")
	log.Info("  2. A one-time join token was generated for agent bootstrap")
	log.Info("  3. The agent used the token to attest itself to the server")
	log.Info("  4. The server registered the agent and issued it an X.509 SVID")
	log.Info("")
	log.Info("Tear down with: scenario-01-simple --down")
}

// findScenarioDir locates the scenario directory relative to the working directory.
func findScenarioDir(relPath string) string {
	// Try current directory first
	if abs, err := filepath.Abs(relPath); err == nil {
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs
		}
	}
	// Fallback: assume we're in repo root
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, relPath)
}
