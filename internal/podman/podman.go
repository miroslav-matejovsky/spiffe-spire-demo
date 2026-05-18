package podman

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
)

// CheckAvailability verifies that podman and podman-compose are installed and
// that the podman daemon/machine is reachable. Returns a user-friendly error
// with remediation steps on failure.
func CheckAvailability() error {
	if _, err := exec.LookPath("podman"); err != nil {
		return fmt.Errorf("podman not found on PATH\n\n" +
			"Podman is required to run scenarios.\n" +
			"Install it from: https://podman.io/docs/installation")
	}

	if _, err := exec.LookPath("podman-compose"); err != nil {
		return fmt.Errorf("podman-compose not found on PATH\n\n" +
			"podman-compose is required to run scenarios.\n" +
			"Install it with: pip install podman-compose")
	}

	// Check if podman daemon/machine is reachable
	cmd := exec.Command("podman", "info")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman is installed but not reachable\n\n%s", podmanMachineHint())
	}

	return nil
}

// podmanMachineHint returns platform-appropriate remediation for unreachable podman.
func podmanMachineHint() string {
	if runtime.GOOS == "linux" {
		return "Check that the podman service is running:\n" +
			"  systemctl --user start podman.socket"
	}
	return "Try:\n" +
		"  podman machine start\n\n" +
		"If this is your first time using Podman:\n" +
		"  podman machine init\n" +
		"  podman machine start"
}

// Compose wraps podman-compose operations for a specific compose project.
type Compose struct {
	// Dir is the working directory containing compose.yml
	Dir string
	// Log is used for verbose output
	Log *logging.Logger
}

// NewCompose creates a Compose instance for the given scenario directory.
func NewCompose(dir string, log *logging.Logger) *Compose {
	return &Compose{Dir: dir, Log: log}
}

// Up starts the specified services (or all if none given).
func (c *Compose) Up(services ...string) error {
	args := []string{"up", "-d"}
	args = append(args, services...)
	_, err := c.run("podman-compose", args...)
	return err
}

// UpNoBuild starts services without rebuilding images.
func (c *Compose) UpNoBuild(services ...string) error {
	args := []string{"up", "-d", "--no-build"}
	args = append(args, services...)
	_, err := c.run("podman-compose", args...)
	return err
}

// Down stops and removes all containers in the composition.
func (c *Compose) Down() error {
	_, err := c.run("podman-compose", "down", "-v", "--remove-orphans")
	return err
}

// Exec runs a command inside a running service container and returns its output.
func (c *Compose) Exec(service string, cmd ...string) (string, error) {
	args := []string{"exec", "-T", service}
	args = append(args, cmd...)
	return c.run("podman-compose", args...)
}

// Logs returns the stdout/stderr logs of a service container.
func (c *Compose) Logs(service string) (string, error) {
	return c.run("podman-compose", "logs", "--no-color", service)
}

// Build builds a container image with the given tag, Containerfile, and context dir.
func Build(tag, containerfile, contextDir string, log *logging.Logger) error {
	args := []string{"build", "-t", tag, "-f", containerfile, contextDir}
	log.Detailf("podman %s", strings.Join(args, " "))
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman build %s: %w", tag, err)
	}
	return nil
}

// RunCompose starts a service using `podman-compose run -d` with extra args.
// Used for services that need runtime arguments (like join tokens).
func (c *Compose) RunDetached(name, service string, args ...string) error {
	cmdArgs := []string{"run", "-d", "--name", name, service}
	cmdArgs = append(cmdArgs, args...)
	_, err := c.run("podman-compose", cmdArgs...)
	return err
}

// RemoveContainer force-removes a container by name. Ignores errors if container
// does not exist.
func RemoveContainer(name string, log *logging.Logger) {
	log.Detailf("removing container %s (if exists)", name)
	cmd := exec.Command("podman", "rm", "-f", name)
	_ = cmd.Run()
}

// ContainerExists checks if a container with the given name exists (running or stopped).
func ContainerExists(name string) bool {
	cmd := exec.Command("podman", "ps", "-a", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// SetEnv sets an environment variable for the compose process.
// This is used for passing join tokens and similar runtime values.
func SetEnv(key, value string) {
	os.Setenv(key, value)
}

// UnsetEnv removes an environment variable.
func UnsetEnv(key string) {
	os.Unsetenv(key)
}

// run executes a command in the compose working directory and returns stdout.
func (c *Compose) run(name string, args ...string) (string, error) {
	c.Log.Detailf("%s %s", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	cmd.Dir = c.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		combined := stdout.String() + stderr.String()
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 125 {
			return combined, fmt.Errorf("%s %s: %w\n\n"+
				"Podman command failed (exit 125). Machine may have stopped.\n"+
				"Try: podman machine start", name, strings.Join(args, " "), err)
		}
		return combined, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, combined)
	}

	return stdout.String(), nil
}
