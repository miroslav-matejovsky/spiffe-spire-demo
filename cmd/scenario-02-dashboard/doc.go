// Command scenario-02-dashboard extends scenario 01 by adding a web dashboard
// that connects to the SPIRE server API over a shared Unix socket volume. It
// demonstrates how external tools can read SPIRE state with gRPC and present it
// as HTML without changing server state.
//
// Usage:
//
//	scenario-02-dashboard [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
