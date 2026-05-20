// Package scenario provides a unified CLI framework for SPIFFE/SPIRE demo scenarios.
//
// Each scenario binary defines a Config with its name, directory, and step functions,
// then calls Run() to handle CLI parsing and dispatch. This eliminates boilerplate
// for argument parsing, compose setup, logging, and teardown across all scenarios.
//
// Supported commands:
//   - up: start the scenario (non-interactive)
//   - down: tear down the scenario
//   - step: start with interactive pauses between steps
//
// Before starting a scenario, Run checks whether any containers from the compose
// project are already running. If they are, it exits with a clear message asking
// the user to run "down" first. This prevents join-token reuse failures and other
// stale-state issues that occur when a previous run was not torn down.
//
// Scenarios may register additional subcommands (e.g. "register", "fetch") via
// the Subcommands field in Config.
//
// Context.ComposeCmd returns a podman-compose command prefix with the -f flag
// pointing to the scenario compose.yml. Scenarios use it in suggestion text so
// that pasted commands work from wherever the user invoked the binary.
package scenario
