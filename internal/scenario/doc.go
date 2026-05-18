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
// Scenarios may register additional subcommands (e.g. "register", "fetch") via
// the Subcommands field in Config.
package scenario
