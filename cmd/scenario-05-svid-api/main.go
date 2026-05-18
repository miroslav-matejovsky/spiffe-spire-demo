package main

import (
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
)

func main() {
	scenario.Run(scenario.Config{
		Name:    "SVID API",
		DirName: "05-svid-api",
		Up:      up,
		Done:    done,
	})
}

func up(ctx *scenario.Context) error {
	err := ctx.Runner.Run(
		"Building Container Images",
		"This scenario has three custom containers to build:\n"+
			"  - Dashboard: web UI for SPIRE server monitoring\n"+
			"  - svid-server: Go HTTPS server using SPIFFE mTLS\n"+
			"  - svid-client: Go HTTPS client using SPIFFE mTLS\n\n"+
			"Both Go services use the go-spiffe library to connect to the Workload API\n"+
			"and automatically fetch/rotate their X.509 SVIDs.",
		func() error {
			dashboardCf := filepath.Join(ctx.RepoRoot, "dashboard", "Containerfile")
			serverCf := filepath.Join(ctx.ScenarioDir, "server", "Containerfile")
			clientCf := filepath.Join(ctx.ScenarioDir, "client", "Containerfile")

			if err := podman.Build("spiffe-spire-demo-dashboard:local", dashboardCf, ctx.RepoRoot, ctx.Log); err != nil {
				return err
			}
			if err := podman.Build("spiffe-spire-demo-svid-server:local", serverCf, ctx.RepoRoot, ctx.Log); err != nil {
				return err
			}
			return podman.Build("spiffe-spire-demo-svid-client:local", clientCf, ctx.RepoRoot, ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Server",
		"Starting the trust domain authority for mirmat.org.",
		func() error {
			return ctx.Compose.Up("spire-server")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for SPIRE Server Health",
		"Polling until the server API is ready.",
		func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting SPIRE Agent",
		"Generate a join token and start the agent. The agent shares a Unix socket\n"+
			"volume with the svid-server and svid-client containers. This socket is\n"+
			"the Workload API endpoint where services fetch their SVIDs.",
		func() error {
			token, err := spirectl.GenerateToken(ctx.Compose, "spire-server", "spiffe://mirmat.org/myagent", ctx.Log)
			if err != nil {
				return err
			}
			podman.SetEnv("SPIRE_AGENT_JOIN_TOKEN", token)
			defer podman.UnsetEnv("SPIRE_AGENT_JOIN_TOKEN")
			return ctx.Compose.Up("spire-agent")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Waiting for Agent Attestation",
		"Agent attests using its join token.",
		func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Registering Workloads",
		"Workload registration maps a process selector to a SPIFFE ID:\n\n"+
			"  svid-server (UID 10001) -> spiffe://mirmat.org/svid-server\n"+
			"  svid-client (UID 10002) -> spiffe://mirmat.org/svid-client\n\n"+
			"The SPIRE agent watches processes on its node. When a process with UID\n"+
			"10001 connects to the Workload API, it receives the svid-server identity.\n"+
			"This is how SPIFFE delivers identity without application changes.",
		func() error {
			agentID, err := spirectl.GetAgentID(ctx.Compose, "spire-server", ctx.Log)
			if err != nil {
				return err
			}
			if err := spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/svid-server", agentID, "unix:uid:10001", ctx.Log); err != nil {
				return err
			}
			return spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/svid-client", agentID, "unix:uid:10002", ctx.Log)
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting mTLS Services",
		"Starting svid-server and svid-client. Both connect to the Workload API\n"+
			"socket, fetch their SVIDs, and establish mTLS communication:\n\n"+
			"  1. svid-server listens on :8443 with its SVID as the server cert\n"+
			"  2. svid-client calls the server, presenting its own SVID\n"+
			"  3. Both verify the peer's identity against the trust bundle\n\n"+
			"The client makes 5 requests and logs the responses.",
		func() error {
			if err := ctx.Compose.UpNoBuild("svid-server"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return ctx.Compose.UpNoBuild("svid-client")
		},
	)
	if err != nil {
		return err
	}

	err = ctx.Runner.Run(
		"Starting Dashboard",
		"Starting the dashboard for monitoring.",
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
	ctx.Log.Step("SVID API scenario is ready!")
	ctx.Log.Info("Dashboard:    http://localhost:8080")
	ctx.Log.Info("Server logs:  podman-compose logs -f svid-server")
	ctx.Log.Info("Client logs:  podman-compose logs -f svid-client")
	ctx.Log.Info("")
	ctx.Log.Info("What is happening:")
	ctx.Log.Info("  - svid-server has SPIFFE ID: spiffe://mirmat.org/svid-server")
	ctx.Log.Info("  - svid-client has SPIFFE ID: spiffe://mirmat.org/svid-client")
	ctx.Log.Info("  - They communicate over mTLS using auto-rotated X.509 SVIDs")
	ctx.Log.Info("  - No certificates were manually configured in the applications")
	ctx.Log.Info("")
	ctx.Log.Info("Tear down with: scenario-05-svid-api down")
}
