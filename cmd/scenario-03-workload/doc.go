// Command scenario-03-workload demonstrates workload identity - the core
// value proposition of SPIFFE. It shows how a workload (any running process)
// can receive an X.509 SVID via the Workload API.
//
// Subcommands:
//
//	(default)  Start the full scenario stack
//	register   Register a workload entry with the SPIRE server
//	fetch      Display the SVID fetched by the workload container
//
// Usage:
//
//	scenario-03-workload [flags]
//	scenario-03-workload register
//	scenario-03-workload fetch
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
