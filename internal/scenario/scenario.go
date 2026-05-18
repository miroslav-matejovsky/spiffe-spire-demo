package scenario

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/step"
)

// Config defines a scenario and its available commands.
type Config struct {
	// Name is the human-readable scenario name (e.g. "Simple", "Dashboard").
	Name string
	// DirName is the directory name under scenarios/ (e.g. "01-simple").
	DirName string
	// Up runs the scenario startup steps. Receives a Context with Runner set.
	Up func(ctx *Context) error
	// Done is called after successful Up. Use for printing completion messages.
	Done func(ctx *Context)
	// Subcommands are extra commands beyond up/down/step (e.g. "register", "fetch").
	Subcommands map[string]Subcommand
}

// Subcommand defines a custom scenario command.
type Subcommand struct {
	// Desc is a short description for usage output.
	Desc string
	// Run executes the subcommand.
	Run func(ctx *Context) error
}

// Context provides shared resources to scenario action functions.
type Context struct {
	Log         *logging.Logger
	Compose     *podman.Compose
	Runner      *step.Runner
	RepoRoot    string
	ScenarioDir string
}

// WaitForDashboard polls a dashboard health URL until it responds 200.
// Warns but does not error on timeout.
func (ctx *Context) WaitForDashboard(url string) {
	ctx.Log.Info("Waiting for dashboard to become ready...")
	client := &http.Client{Timeout: 2 * time.Second}
	for attempt := 1; attempt <= 30; attempt++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ctx.Log.Ok("Dashboard is ready.")
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		ctx.Log.Detailf("attempt %d/30: waiting for dashboard...", attempt)
		time.Sleep(2 * time.Second)
	}
	ctx.Log.Warn("Dashboard did not respond in time -- it may still be starting.")
}

// Run parses CLI arguments and dispatches to the appropriate command.
// Exit codes: 0 on success, 1 on failure.
func Run(cfg Config) {
	if len(os.Args) < 2 {
		printUsage(cfg)
		os.Exit(1)
	}

	command := os.Args[1]

	// Parse --verbose flag for the subcommand
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	verbose := fs.Bool("verbose", false, "show detailed debug output")
	fs.Parse(os.Args[2:])

	log := logging.New(*verbose)

	// Preflight: verify podman and podman-compose are available
	if err := podman.CheckAvailability(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		log.Error("Could not find repository root (no go.mod found in parent directories).")
		os.Exit(1)
	}

	scenarioDir := filepath.Join(repoRoot, "scenarios", cfg.DirName)
	compose := podman.NewCompose(scenarioDir, log)

	ctx := &Context{
		Log:         log,
		Compose:     compose,
		RepoRoot:    repoRoot,
		ScenarioDir: scenarioDir,
	}

	switch command {
	case "up":
		if compose.HasRunningContainers() {
			log.Errorf("Scenario '%s' is already running.", cfg.Name)
			log.Errorf("Tear it down first:  %s down", filepath.Base(os.Args[0]))
			os.Exit(1)
		}
		ctx.Runner = step.New(log, false)
		if err := cfg.Up(ctx); err != nil {
			os.Exit(1)
		}
		if cfg.Done != nil {
			cfg.Done(ctx)
		}

	case "step":
		if compose.HasRunningContainers() {
			log.Errorf("Scenario '%s' is already running.", cfg.Name)
			log.Errorf("Tear it down first:  %s down", filepath.Base(os.Args[0]))
			os.Exit(1)
		}
		ctx.Runner = step.New(log, true)
		if err := cfg.Up(ctx); err != nil {
			os.Exit(1)
		}
		if cfg.Done != nil {
			cfg.Done(ctx)
		}

	case "down":
		log.Step(fmt.Sprintf("Tearing down %s scenario...", cfg.Name))
		if err := compose.Down(); err != nil {
			log.Errorf("teardown failed: %v", err)
			os.Exit(1)
		}
		log.Ok(fmt.Sprintf("%s scenario stopped.", cfg.Name))

	default:
		if sub, ok := cfg.Subcommands[command]; ok {
			ctx.Runner = step.New(log, false)
			if err := sub.Run(ctx); err != nil {
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
			printUsage(cfg)
			os.Exit(1)
		}
	}
}

func printUsage(cfg Config) {
	bin := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [--verbose]\n\n", bin)
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  %-10s Start the scenario\n", "up")
	fmt.Fprintf(os.Stderr, "  %-10s Tear down the scenario\n", "down")
	fmt.Fprintf(os.Stderr, "  %-10s Start with interactive pauses between steps\n", "step")
	for name, sub := range cfg.Subcommands {
		fmt.Fprintf(os.Stderr, "  %-10s %s\n", name, sub.Desc)
	}
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  --verbose  Show detailed debug output\n")
}

// findRepoRoot walks up from cwd looking for go.mod.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found above %s", cwd)
}
