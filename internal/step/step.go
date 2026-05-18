package step

import (
	"bufio"
	"fmt"
	"os"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
)

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

// Run executes a single named step. It prints the explanation, optionally
// pauses for user input, then runs the action function.
func (r *Runner) Run(name, explanation string, action func() error) error {
	r.stepCount++

	// Print step header
	fmt.Fprintf(os.Stderr, "\n\033[1;36m=== Step %d: %s ===\033[0m\n", r.stepCount, name)

	// Print explanation if provided
	if explanation != "" {
		fmt.Fprintf(os.Stderr, "\033[37m%s\033[0m\n", explanation)
	}

	// In step mode, wait for user to press Enter
	if r.stepMode {
		fmt.Fprintf(os.Stderr, "\n\033[33m[Press Enter to continue]\033[0m ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
	}

	fmt.Fprintln(os.Stderr)

	// Execute the action
	if err := action(); err != nil {
		r.log.Errorf("step %q failed: %v", name, err)
		return fmt.Errorf("step %q: %w", name, err)
	}

	return nil
}

// Log returns the underlying logger for use in action functions.
func (r *Runner) Log() *logging.Logger {
	return r.log
}
