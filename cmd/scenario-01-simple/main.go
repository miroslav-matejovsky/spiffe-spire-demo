package main

import (
	"fmt"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "Simple",
		DirName: "01-simple",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	var token string
	var agentList string

	err := ctx.Runner.Run(
		"Starting SPIRE Server",
		"The SPIRE Server is the central authority in a SPIFFE trust domain.\n"+
			"It manages identity registration, signs X.509 SVIDs and JWT SVIDs,\n"+
			"and maintains the trust bundle that all workloads use for verification.\n\n"+
			"We start it as a container using podman-compose.",
		func() error {
			return ctx.Compose.Up("spire-server")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for SPIRE Server Health",
		"The SPIRE server needs a moment to initialize its datastore, load plugins,\n"+
			"and start listening on its API socket. We poll its healthcheck endpoint\n"+
			"until it reports ready.",
		func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Generating Join Token",
		"A join token is a one-time-use secret that allows a SPIRE agent to prove\n"+
			"its identity to the server during initial attestation. The server generates\n"+
			"the token and associates it with a SPIFFE ID for the agent.\n\n"+
			"Join tokens are the simplest attestation method. In production, you would\n"+
			"use hardware-based attestation (TPM, cloud instance identity, etc.).",
		func() error {
			var err error
			token, err = spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	agentContainer := "simple-spire-agent"
	err = ctx.Runner.Run(
		"Starting SPIRE Agent",
		"The SPIRE Agent runs on each node (or in each container) that hosts workloads.\n"+
			"It connects to the SPIRE Server using the join token for initial attestation.\n"+
			"Once attested, the agent can request SVIDs on behalf of its workloads.\n\n"+
			"The agent exposes the Workload API - a local Unix socket that workloads\n"+
			"connect to for fetching their identities.",
		func() error {
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
		"After the agent starts, it presents the join token to the server.\n"+
			"The server validates the token (single-use), creates an agent record,\n"+
			"and issues the agent its own SVID. This process is called attestation.\n\n"+
			"We poll the server's agent list until we see our agent registered.",
		func() error {
			var err error
			agentList, err = spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	_ = agentList
	return nil
}

func done(ctx *scenario.Context) {
	ctx.Log.Step("Simple scenario is ready!")
	ctx.Log.Info("Server: simple-spire-server")
	ctx.Log.Info("Agent:  simple-spire-agent")
	ctx.Log.Info("")
	ctx.Log.Info("What just happened:")
	ctx.Log.Info("  1. SPIRE server started and initialized the trust domain 'mirmat.org'")
	ctx.Log.Info("  2. A one-time join token was generated for agent bootstrap")
	ctx.Log.Info("  3. The agent used the token to attest itself to the server")
	ctx.Log.Info("  4. The server registered the agent and issued it an X.509 SVID")
	ctx.Log.Info("")
	ctx.Log.Info(fmt.Sprintf("Tear down with: %s down", "scenario-01-simple"))
}
