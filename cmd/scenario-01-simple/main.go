package main

import (
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
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

	err := ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server",
		Explain: "SPIRE Server is control plane for trust domain.\n" +
			"In scenarios/01-simple/spire/server/server.conf, trust_domain = \"mirmat.org\"\n" +
			"sets root for every SPIFFE ID, like spiffe://mirmat.org/...\n" +
			"Same file binds server on port 8081, uses DataStore \"sql\" with SQLite,\n" +
			"and enables NodeAttestor \"join_token\" for first agent bootstrap.",
		Action: func() error {
			return ctx.Compose.Up("spire-server")
		},
		Observe: "Server initialized trust domain mirmat.org.\n" +
			"SQLite datastore under /opt/spire/data/server now holds SPIRE state.\n" +
			"Server is listening on 8081 for agent traffic and admin API socket\n" +
			"/tmp/spire-server/private/api.sock is ready for spire-server commands.\n" +
			"Try: " + ctx.ComposeCmd() + " logs spire-server",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for SPIRE Server Health",
		Explain: "Healthcheck matters because process can be up but not ready.\n" +
			"Server still must load plugins from server.conf, open SQLite datastore,\n" +
			"create or load signing keys with KeyManager \"memory\", and build CA state.\n" +
			"Until healthcheck passes, server cannot sign SVIDs or accept agents.",
		Action: func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "Server passed healthcheck.\n" +
			"Its internal CA is ready to sign X.509-SVIDs for agents and workloads.\n" +
			"join_token NodeAttestor is loaded and waiting for agent bootstrap.\n" +
			"Next admin calls over /tmp/spire-server/private/api.sock will work.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Generating Join Token",
		Explain: "Join token is one-time secret for first agent attestation.\n" +
			"Server creates token, stores it, and ties it to agent SPIFFE ID\n" +
			"spiffe://mirmat.org/myagent with limited lifetime.\n" +
			"Good for first demo only. Real systems use stronger node proof like TPM,\n" +
			"cloud metadata, or other platform attestors.",
		Action: func() error {
			var err error
			token, err = spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			return err
		},
		Observe: "Token generated and stored in server datastore.\n" +
			"Token is single-use, so second presentation will fail.\n" +
			"Server associated it with SPIFFE ID spiffe://mirmat.org/myagent.\n" +
			"Agent will present token during startup to prove it is allowed in.",
	})
	if err != nil {
		return err
	}

	agentContainer := "simple-spire-agent"
	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Agent",
		Explain: "SPIRE Agent is node-local identity broker for workloads.\n" +
			"In scenarios/01-simple/spire/agent/agent.conf, server_address = \"spire-server\"\n" +
			"and server_port = \"8081\" point at server, while trust_domain must match\n" +
			"mirmat.org. socket_path = \"/opt/spire/sockets/workload_api.sock\" exposes\n" +
			"Workload API. insecure_bootstrap = true keeps first demo simple only.",
		Action: func() error {
			podman.RemoveContainer(agentContainer, ctx.Log)
			return ctx.Compose.RunDetached(agentContainer, "spire-agent",
				"-config", "/opt/spire/conf/agent/agent.conf",
				"-joinToken", token,
			)
		},
		Observe: "Agent started and is dialing spire-server:8081.\n" +
			"It presents join token to NodeAttestor \"join_token\" for bootstrap.\n" +
			"insecure_bootstrap = true means agent trusts server bundle on first contact\n" +
			"without preloaded bundle, which is fine for demo and bad for production.\n" +
			"Try: " + ctx.ComposeCmd() + " logs spire-agent",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Agent Attestation",
		Explain: "Attestation is moment server decides agent is trusted node.\n" +
			"Server validates join token, checks single-use rule, and creates agent record.\n" +
			"Then it issues agent its own X.509-SVID inside trust domain mirmat.org.\n" +
			"Afterward, agent can serve Workload API and later use WorkloadAttestor\n" +
			"\"unix\" to identify local workloads by Unix selectors.",
		Action: func() error {
			var err error
			agentList, err = spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
		Observe: "Agent attested.\n" +
			"Server assigned SPIFFE ID like spiffe://mirmat.org/spire/agent/join_token/<uuid>.\n" +
			"Agent now holds valid X.509-SVID and trust bundle for mirmat.org.\n" +
			"It can serve Workload API to local workloads over workload_api.sock.\n" +
			"Try: " + ctx.ComposeCmd() + " exec spire-server /opt/spire/bin/spire-server agent list",
	})
	if err != nil {
		return err
	}

	_ = agentList
	return nil
}

func done(ctx *scenario.Context) {
	composeCmd := ctx.ComposeCmd()
	ctx.Log.Step("Simple scenario ready")
	ctx.Log.Info("Server: simple-spire-server")
	ctx.Log.Info("Agent:  simple-spire-agent")
	ctx.Log.Info("")
	ctx.Log.Info("What you learned:")
	ctx.Log.Info("  1. trust_domain mirmat.org is root for every SPIFFE ID")
	ctx.Log.Info("  2. Server on 8081 signs identities and stores state in SQLite")
	ctx.Log.Info("  3. Join token bootstraps first agent and becomes invalid after use")
	ctx.Log.Info("  4. Agent now has X.509-SVID and serves Workload API on local socket")
	ctx.Log.Info("")
	ctx.Log.Info("Try next:")
	ctx.Log.Info("  - " + composeCmd + " logs spire-server")
	ctx.Log.Info("  - " + composeCmd + " logs spire-agent")
	ctx.Log.Info("  - " + composeCmd + " exec spire-server /opt/spire/bin/spire-server agent list")
	ctx.Log.Info("Tear down with: scenario-01-simple down")
}
