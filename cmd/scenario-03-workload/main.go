package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/scenario"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/spirectl"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
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
	err := ctx.Runner.RunStep(step.Step{
		Name: "Building Dashboard Image",
		Explain: lines(
			"Dashboard is helper for learning, not identity source itself.",
			"It talks to SPIRE server API and shows agents, entries, bundles, and status.",
			"We build image first so later browser checks are ready when stack comes up.",
		),
		Action: func() error {
			return podman.BuildDashboard(ctx.RepoRoot, ctx.Log)
		},
		Observe: lines(
			"Dashboard image is ready in local container cache.",
			"No SPIRE state changed yet. Only UI image was prepared.",
			"Later open http://localhost:8080 to inspect agents, entries, and trust data.",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Server",
		Explain: lines(
			"SPIRE server is control plane for trust domain mirmat.org.",
			"It stores registration entries: SPIFFE ID, parent ID, and selectors that must match.",
			"Agents ask server which workloads on their node may receive identities.",
			"Server starts empty. No workload policy exists yet.",
		),
		Action: func() error {
			return ctx.Compose.Up("spire-server")
		},
		Observe: lines(
			"Server is running and owns registration database for trust domain.",
			"No workload entries are registered yet, so workloads would still be denied.",
			"Entry creation happens later in register subcommand.",
			"After dashboard starts, check Entries page and expect empty list.",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for SPIRE Server Health",
		Explain: lines(
			"Healthcheck waits until server API is ready to answer commands.",
			"Join tokens, agent attestation, and entry writes all depend on this API.",
			"We wait now so later steps do not race server startup.",
		),
		Action: func() error {
			return spirectl.Healthcheck(ctx.Compose, "spire-server", ctx.Log)
		},
		Observe: lines(
			"Server is ready to accept registration entries and agent connections.",
			"Next join token can be issued safely.",
			"Try: podman logs workload-spire-server",
			"If health fails here, inspect server config and storage startup first.",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting SPIRE Agent with Join Token",
		Explain: lines(
			"SPIRE agent runs beside workloads and exposes Workload API socket.",
			"In agent.conf socket path is /opt/spire/sockets/workload_api.sock.",
			"That path lives on shared-socket volume, mounted by agent and workload containers.",
			"Workloads never talk to server directly. They ask local agent over this socket.",
			"Join token lets server attest this agent as trusted node in demo.",
		),
		Action: func() error {
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
		Observe: lines(
			"Agent started and is creating Workload API socket on shared volume.",
			"Any container that mounts shared-socket can reach socket file.",
			"Workloads that connect to this socket will be attested by unix WorkloadAttestor.",
			"Try: podman exec workload-spire-agent ls -l /opt/spire/sockets",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Waiting for Agent Attestation",
		Explain: lines(
			"Join token proves agent is allowed to join trust domain.",
			"After attestation, server issues agent its own SPIFFE ID and trusts workload claims from it.",
			"Only then can agent answer Workload API requests for local processes.",
		),
		Action: func() error {
			_, err := spirectl.WaitForAgent(ctx.Compose, "spire-server", "join_token", ctx.Log)
			return err
		},
		Observe: lines(
			"Agent is attested and connected to server.",
			"Now it can serve Workload API to local workloads.",
			"But no workload entries exist yet, so SVID requests would still be denied.",
			"Next step starts workload so you can see deny-then-allow behavior.",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Workload Container",
		Explain: lines(
			"Workload container stands in for real app that wants identity.",
			"It mounts shared-socket volume, so same /opt/spire/sockets/workload_api.sock is visible inside workload.",
			"It also joins agent PID namespace with pid: \"container:workload-spire-agent\".",
			"PID namespace sharing lets unix WorkloadAttestor map API caller to real process metadata.",
			"Workload runs spire-agent api watch, which keeps asking for SVIDs and prints updates to logs.",
			"Socket access alone is not enough. Matching registration policy is still required.",
		),
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("workload"); err != nil {
				return err
			}
			time.Sleep(3 * time.Second)
			return nil
		},
		Observe: lines(
			"Workload container is running and calling Workload API now.",
			"Agent sees request, but no registration entry matches yet, so request is denied.",
			"Run scenario-03-workload register to add policy for unix:uid:0.",
			"Try: podman logs workload-workload",
			"Same watch command will later show automatic SVID refresh events.",
		),
	})
	if err != nil {
		return err
	}

	err = ctx.Runner.RunStep(step.Step{
		Name: "Starting Dashboard",
		Explain: lines(
			"Dashboard is read-only teaching tool for SPIRE server state.",
			"It shows agents, registration entries, and other control-plane data in browser.",
			"Use it to see when policy changes take effect after register step.",
		),
		Action: func() error {
			if err := ctx.Compose.UpNoBuild("dashboard"); err != nil {
				return err
			}
			ctx.WaitForDashboard("http://127.0.0.1:8080/health")
			return nil
		},
		Observe: lines(
			"Dashboard is up at http://localhost:8080.",
			"Open Entries page. It should be empty right now.",
			"After you run scenario-03-workload register, new workload entry will appear.",
			"Compare dashboard view with CLI output to connect policy to behavior.",
		),
	})
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

// register creates workload policy on SPIRE server.
func register(ctx *scenario.Context) error {
	return ctx.Runner.RunStep(step.Step{
		Name: "Registering Workload",
		Explain: lines(
			"Registration entry is SPIRE policy record for workload identity.",
			"SPIFFE ID says which identity to mint: spiffe://mirmat.org/myworkload.",
			"Parent ID says which attested agent may present workloads for this entry.",
			"Selector unix:uid:0 says any process seen as UID 0 in agent PID namespace matches.",
			"In this scenario workload runs as root, so unix attestor will produce that selector.",
			"Until entry exists, workload has socket access but no authority to get identity.",
		),
		Action: func() error {
			agentID, err := spirectl.GetAgentID(ctx.Compose, "spire-server", ctx.Log)
			if err != nil {
				return fmt.Errorf("no attested agent found - run 'up' first: %w", err)
			}
			return spirectl.CreateEntry(ctx.Compose, "spire-server",
				"spiffe://mirmat.org/myworkload", agentID, "unix:uid:0", ctx.Log,
				"Demo workload watching for SVIDs")
		},
		Observe: lines(
			"Entry was created in server database.",
			"SPIRE agent will now match workload's unix:uid:0 selector under attested parent agent.",
			"Workload should receive spiffe://mirmat.org/myworkload X.509-SVID automatically.",
			"Check dashboard Entries page, or run: scenario-03-workload fetch",
			"You can inspect raw policy with: podman exec workload-spire-server /opt/spire/bin/spire-server entry show",
		),
	})
}

// fetch displays SVID data streamed by workload's api watch process.
func fetch(ctx *scenario.Context) error {
	return ctx.Runner.RunStep(step.Step{
		Name: "Fetching Workload SVID",
		Explain: lines(
			"Workload is already running spire-agent api watch against Workload API socket.",
			"After registration entry matches, agent streams X.509-SVID to workload without restart.",
			"We read container logs to see SPIFFE ID, validity window, and CA chain details.",
			"SVIDs are short-lived, and SPIRE rotates them before expiry.",
		),
		Action: func() error {
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
					return nil
				}
				ctx.Log.Detailf("attempt %d/30: waiting for SVID...", attempt)
				time.Sleep(2 * time.Second)
			}

			ctx.Log.Error("Workload has not received an SVID yet.")
			ctx.Log.Info("Make sure 'scenario-03-workload register' completed successfully.")
			return fmt.Errorf("SVID not found after 30 attempts")
		},
		Observe: lines(
			"Output above came from live Workload API watch stream.",
			"You should see SPIFFE ID spiffe://mirmat.org/myworkload and certificate validity window.",
			"SPIRE will refresh SVID before expiration, and watch command will print updated data again.",
			"Run: podman logs -f workload-workload to watch future rotations.",
			"No app restart is needed. Workload identity lifecycle is automatic.",
		),
	})
}

// lines joins educational text into blocks shown by RunStep.
func lines(parts ...string) string {
	return strings.Join(parts, "\n")
}
