package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "Workload",
		DirName: "03-workload",
		Up:      up,
		Done:    done,
		Subcommands: map[string]scenario.Subcommand{
			"register": {Desc: "Register workload entry with SPIRE server", Run: register},
			"fetch":    {Desc: "Fetch and display workload SVID from container logs", Run: fetch},
		},
	})
}

func up(ctx *scenario.Context) error {
	err := ctx.Runner.Run(
		"Building Dashboard Image",
		"Building the dashboard container for monitoring SPIRE state.",
		func() error {
			containerfile := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			return podman.Build("spiffe-spire-demo-dashboard:local", containerfile, ctx.RepoRoot, ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Server",
		"Starting the SPIRE server - the trust domain authority for mirmat.org.",
		func() error {
			return ctx.Compose.Up("spire-server")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling server healthcheck until ready.",
		func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Agent with Join Token",
		"Generate a join token, then start the agent. The agent exposes the\n"+
			"Workload API socket that workloads connect to for identity.",
		func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			agentContainer := "workload-spire-agent"
			podman.RemoveContainer(agentContainer, ctx.Log)
			return ctx.Compose.RunDetached(agentContainer, "spire-agent",
				"-config", "/opt/spire/conf/agent/agent.conf",
				"-joinToken", token,
			)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for Agent Attestation",
		"The agent attests to the server using the join token.",
		func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Workload Container",
		"The workload container shares the Workload API socket with the SPIRE agent\n"+
			"via a named volume. Once a workload entry is registered for this container,\n"+
			"it will automatically receive an X.509 SVID.",
		func() error {
			if err := ctx.Compose.UpNoBuild("workload"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return nil
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Dashboard",
		"Starting the web dashboard for monitoring.",
		func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func done(ctx *scenario.Context) {
	ctx.Log.Step("Workload scenario is ready!")
	ctx.Log.Info("Dashboard: http://localhost:8080")
	ctx.Log.Info("")
	ctx.Log.Info("Next steps:")
	ctx.Log.Info("  1. Register a workload: scenario-03-workload register")
	ctx.Log.Info("  2. Fetch the SVID:      scenario-03-workload fetch")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-03-workload down")
}

// register registers a workload entry with the SPIRE server.
func register(ctx *scenario.Context) error {
	return ctx.Runner.Run(
		"Registering Workload",
		"A workload registration entry tells SPIRE: 'any process matching this\n"+
			"selector should receive this SPIFFE ID'. We use unix:uid:0 (root) as\n"+
			"the selector, meaning any root process on the agent node gets the ID.\n\n"+
			"SPIFFE ID: spiffe://mirmat.org/myworkload\n"+
			"Selector:  unix:uid:0 (root user inside the container)",
		func() error {
			agentID, err := spirectl.GetAgentID(ctx.Compose, "spire-server", ctx.Log)
			if err != nil {
				return fmt.Errorf("no attested agent found - run 'up' first: %w", err)
			}
			return spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/myworkload", agentID, "unix:uid:0", ctx.Log)
		},
	)
}

// fetch displays the SVID fetched by the workload container.
// The workload runs "spire-agent api watch" which writes SVID details to its
// container logs rather than a file.
func fetch(ctx *scenario.Context) error {
	ctx.Log.Step("Fetching workload SVID...")
	ctx.Log.Info("Reading logs from the workload container to find SVID details...")

	for attempt := 1; attempt <= 30; attempt++ {
		output, err := ctx.Compose.Logs("workload")
		if err == nil && strings.Contains(output, "Received 1 svid") {
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "Received") ||
					strings.Contains(line, "SPIFFE ID") ||
					strings.Contains(line, "SVID Valid") ||
					strings.Contains(line, "CA #") {
					fmt.Println(line)
				}
			}
			ctx.Log.Ok("SVID fetched successfully!")
			ctx.Log.Info("The output above shows the X.509-SVID issued to the workload.")
			return nil
		}
		ctx.Log.Detailf("attempt %d/30: waiting for SVID...", attempt)
		time.Sleep(2 * time.Second)
	}

	ctx.Log.Error("Workload has not received an SVID yet.")
	ctx.Log.Info("Make sure 'scenario-03-workload register' completed successfully.")
	return fmt.Errorf("SVID not found after 30 attempts")
}
