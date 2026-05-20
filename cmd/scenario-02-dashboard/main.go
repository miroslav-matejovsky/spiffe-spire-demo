package main

import (
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
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
	err := ctx.Runner.RunStep(step.Step{
		Name: "Building Dashboard Image",
		Explain: "Dashboard is Go web app, not SPIRE plugin.\n" +
			"It opens gRPC connection to SPIRE Server API and reads agents, entries, and bundles.\n" +
			"Multi-stage build compiles Go binary first, then copies only binary into small runtime image.\n" +
			"Small image keeps demo simple and shows how external tools can package SPIRE API clients.",
		Sources: []step.Source{
			ctx.RepoSrc("dashboard/Containerfile", 0, "multi-stage build for dashboard"),
			ctx.RepoSrc("cmd/dashboard/main.go", 0, "dashboard entry point and gRPC setup"),
			ctx.RepoSrc("cmd/dashboard/handlers.go", 0, "HTTP handlers for SPIRE API data"),
		},
		Action: func() error {
			return podman.BuildDashboard(ctx.RepoRoot, ctx.Log)
		},
		Observe: "Image built. Dashboard container can now start from local image.\n" +
			"When it runs, it will connect to unix:///tmp/spire-server/private/api.sock.\n" +
			"Socket path comes from shared volume between SPIRE server and dashboard containers.\n" +
			"Dashboard will translate gRPC replies into HTML pages for browser.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server",
		Explain: "SPIRE Server stays trust authority for mirmat.org and signs trust data for domain.\n" +
			"New part in scenario 02 is management API socket at /tmp/spire-server/private/api.sock.\n" +
			"scenarios/02-dashboard/compose.yml mounts volume spire-server-socket into server at that path.\n" +
			"Same volume will later be mounted into dashboard so both containers see same Unix socket.",
		Sources: []step.Source{
			ctx.Src("compose.yml", 10, "spire-server-socket volume mount"),
			ctx.Src("compose.yml", 22, "dashboard service shares same volume"),
			ctx.Src("spire/server/server.conf", 4, "socket_path for API"),
		},
		Action: func() error {
			return ctx.Compose.Up("spire-server")
		},
		Observe: "Server running inside container dashboard-spire-server.\n" +
			"SPIRE creates API socket at /tmp/spire-server/private/api.sock inside mounted volume.\n" +
			"Dashboard will not talk over TCP. It will read server state through that Unix socket.\n" +
			"Next step waits until server fully initializes and accepts gRPC calls.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for SPIRE Server Health",
		Explain: "Ready means more than container started.\n" +
			"SPIRE Server must open datastore, initialize CA state, and load node attestor plugins.\n" +
			"It also must create API socket and begin accepting gRPC requests on it.\n" +
			"We poll health until dashboard can safely query live state.",
		Action: func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: "Server healthy. Core services finished startup work.\n" +
			"API socket is ready for gRPC connections from dashboard container.\n" +
			"If dashboard started before this point, socket connect could fail or return incomplete state.\n" +
			"Now server can answer read requests about agents, entries, and trust bundle.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Agent with Join Token",
		Explain: "Join token flow bootstraps new agent without preinstalled machine identity.\n" +
			"Server makes one-time token and binds it to SPIFFE ID spiffe://mirmat.org/myagent.\n" +
			"We pass token on agent startup, so agent can prove it received server-issued secret.\n" +
			"Scenario combines token generation and agent launch into one teaching step.",
		Action: func() error {
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
		Observe: "Agent process started and is trying attestation with join token.\n" +
			"Server has not yet confirmed identity, so dashboard may not show agent immediately.\n" +
			"After attestation succeeds, agent record will appear on dashboard Agents page.\n" +
			"Next step waits until server lists agent as trusted node.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Agent Attestation",
		Explain: "Attestation proves node running agent is allowed to join trust domain.\n" +
			"Server checks token, records agent identity, and issues agent its own SVID.\n" +
			"From then on, workloads behind agent can ask for identities through Workload API.\n" +
			"We wait until server reports agent in its managed agent list.",
		Action: func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
		Observe: "Agent attested. SPIRE Server now trusts node represented by dashboard-spire-agent.\n" +
			"Open dashboard Agents page to see same result in UI instead of spire-server CLI output.\n" +
			"You should see SPIFFE ID spiffe://mirmat.org/myagent and join_token attestor type.\n" +
			"Dashboard is useful when you want visual state, not raw command output.",
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: "Dashboard is external read-only tool for SPIRE state.\n" +
			"It mounts same spire-server-socket volume, so /tmp/spire-server/private/api.sock appears inside dashboard container too.\n" +
			"Go code in dashboard opens gRPC client on Unix socket, calls SPIRE Server API, then renders HTML.\n" +
			"It does not modify SPIRE state in demo. It only reads and presents it in browser.",
		Sources: []step.Source{
			ctx.Src("compose.yml", 31, "dashboard mounts spire-server-socket"),
			ctx.RepoSrc("cmd/dashboard/main.go", 0, "gRPC connection to SPIRE API"),
			ctx.RepoSrc("cmd/dashboard/handlers.go", 0, "handlers query agents, entries, bundles"),
		},
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
		Observe: "Dashboard running at http://localhost:8080.\n" +
			"Open Overview page to inspect server health and object counts.\n" +
			"Open Agents page to see attested agent, or Trust Bundle page to inspect X.509 authorities for mirmat.org.\n" +
			"Each page comes from live gRPC reads over shared Unix socket, then rendered as HTML.",
	})
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
