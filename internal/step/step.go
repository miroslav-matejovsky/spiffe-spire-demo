package step

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
)

// Source points to a file in the codebase relevant to a step.
// Printed as a clickable link in VS Code terminal after the Explain text.
type Source struct {
	Path  string // absolute path to file
	Line  int    // line number (0 means whole file)
	Label string // short description of what to look at
}

// Step defines a single educational step in a scenario.
type Step struct {
	// Name is a short heading displayed in the step banner.
	Name string
	// Explain is shown BEFORE the action runs. Describes what will happen
	// and why, references config files, and calls out SPIFFE/SPIRE concepts.
	Explain string
	// Action performs the actual work (start container, generate token, etc.).
	Action func() error
	// Observe is shown AFTER the action succeeds. Explains what SPIRE did,
	// what output to look at, and suggests verification commands.
	Observe string
	// Sources lists codebase files relevant to this step. Each entry is
	// printed as a VS Code-clickable path:line link after the Explain text,
	// so developers can click and investigate the code during the scenario.
	Sources []Source
}

// Runner executes a sequence of interactive steps.
type Runner struct {
	log       *logging.Logger
	stepMode  bool
	stepCount int
}

// New creates a Runner. When stepMode is true, the runner pauses between steps
// and waits for the user to press Enter.
func New(log *logging.Logger, stepMode bool) *Runner {
	return &Runner{
		log:      log,
		stepMode: stepMode,
	}
}

// RunStep executes a Step with full educational output. It prints the
// explanation before the action and the observation after it succeeds.
// In step mode, it pauses before the action and after the observation.
func (r *Runner) RunStep(s Step) error {
	r.stepCount++

	fmt.Fprintf(os.Stderr, "\n\033[1;36m=== Step %d: %s ===\033[0m\n", r.stepCount, s.Name)

	if s.Explain != "" {
		fmt.Fprintf(os.Stderr, "\033[37m%s\033[0m\n", s.Explain)
	}

	if len(s.Sources) > 0 {
		fmt.Fprintf(os.Stderr, "\n\033[90m  Code:\033[0m\n")
		for _, src := range s.Sources {
			path := src.Path
			if cwd, err := os.Getwd(); err == nil {
				if rel, err := filepath.Rel(cwd, path); err == nil {
					path = rel
				}
			}
			if src.Line > 0 {
				fmt.Fprintf(os.Stderr, "\033[90m    %s:%d", path, src.Line)
			} else {
				fmt.Fprintf(os.Stderr, "\033[90m    %s", path)
			}
			if src.Label != "" {
				fmt.Fprintf(os.Stderr, "  - %s", src.Label)
			}
			fmt.Fprintf(os.Stderr, "\033[0m\n")
		}
	}

	if r.stepMode {
		fmt.Fprintf(os.Stderr, "\n\033[33m[Press Enter to continue]\033[0m ")
		scanLine()
	}

	fmt.Fprintln(os.Stderr)

	if err := s.Action(); err != nil {
		r.log.Errorf("step %q failed: %v", s.Name, err)
		return fmt.Errorf("step %q: %w", s.Name, err)
	}

	if s.Observe != "" {
		fmt.Fprintf(os.Stderr, "\n\033[1;32m--- What happened ---\033[0m\n")
		fmt.Fprintf(os.Stderr, "\033[32m%s\033[0m\n", s.Observe)

		if r.stepMode {
			fmt.Fprintf(os.Stderr, "\n\033[33m[Press Enter to continue]\033[0m ")
			scanLine()
		}
	}

	return nil
}

// Run executes a single named step. It prints the explanation, optionally
// pauses for user input, then runs the action function.
// Kept for backward compatibility. Prefer RunStep for educational scenarios.
func (r *Runner) Run(name, explanation string, action func() error) error {
	return r.RunStep(Step{
		Name:    name,
		Explain: explanation,
		Action:  action,
	})
}

// Log returns the underlying logger for use in action functions.
func (r *Runner) Log() *logging.Logger {
	return r.log
}

// scanLine reads one line from stdin. Used for step-mode pauses.
func scanLine() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}
