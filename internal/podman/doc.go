// Package podman provides a typed Go wrapper around podman and podman-compose
// commands. It uses os/exec to shell out to the podman CLI tools.
//
// This package exists because the SPIFFE/SPIRE demo scenarios need to:
//   - Start and stop container compositions (podman-compose up/down)
//   - Execute commands inside running containers (podman-compose exec)
//   - Build container images (podman build)
//   - Query container status (podman ps)
//   - Verify podman availability before running scenarios (preflight check)
//   - Detect whether a scenario is already running (HasRunningContainers)
//
// Rather than calling os/exec directly throughout the scenario code, this package
// provides a structured API that handles working directories, output capture,
// error wrapping, and verbose logging.
//
// The Compose struct is the primary entry point. Create one with a working directory
// (the scenario folder containing compose.yml) and use its methods to orchestrate
// containers. The constructor reads the compose.yml to populate ProjectName, which
// enables HasRunningContainers to detect stale scenario state.
//
// CheckAvailability should be called before any scenario operations. It verifies
// that podman and podman-compose are installed and that the podman machine/daemon
// is reachable, returning user-friendly error messages on failure.
package podman
