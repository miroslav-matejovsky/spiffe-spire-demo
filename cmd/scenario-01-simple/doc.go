// Command scenario-01-simple demonstrates the most basic SPIFFE/SPIRE setup:
// a single SPIRE server and a single SPIRE agent connected via join token attestation.
//
// This is the starting point for understanding SPIRE. It shows:
//   - How a SPIRE server starts and becomes the trust domain authority
//   - How a join token is generated for agent bootstrap
//   - How an agent uses that token to attest its identity to the server
//   - How the server registers the agent in its datastore
//
// Usage:
//
//	scenario-01-simple [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
