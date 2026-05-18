package main

import (
	"path/filepath"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "Dashboard",
		DirName: "02-dashboard",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	err := ctx.Runner.Run(
		"Building Dashboard Image",
		"The dashboard is a Go web application that connects to the SPIRE server's\n"+
			"gRPC API. It displays agents, registered entries, and trust bundle info.\n\n"+
			"We build it as a container image using a multi-stage Containerfile.",
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
		"Same as scenario 01 - the SPIRE server is the trust domain authority.",
		func() error {
			return ctx.Compose.Up("spire-server")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling the server healthcheck until it is ready to accept API calls.",
		func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Agent with Join Token",
		"We generate a join token and start the agent in one step.\n"+
			"The agent will attest to the server automatically on startup.",
		func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			agentContainer := "dashboard-spire-agent"
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
		"The agent presents its join token to the server for attestation.",
		func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Dashboard",
		"The dashboard container connects to the SPIRE server's admin API socket.\n"+
			"It is mounted via a shared volume so the dashboard can query agents,\n"+
			"entries, and bundles in real-time.\n\n"+
			"Dashboard URL: http://localhost:8080",
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
	ctx.Log.Step("Dashboard scenario is ready!")
	ctx.Log.Info("Dashboard: http://localhost:8080")
	ctx.Log.Info("")
	ctx.Log.Info("What you can explore:")
	ctx.Log.Info("  - Overview page shows server health, agent count, entry count")
	ctx.Log.Info("  - Agents page shows the attested agent and its SPIFFE ID")
	ctx.Log.Info("  - Trust Bundle page shows the X.509 authorities for the domain")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-02-dashboard down")
}
