// Command scenario-01-simple demonstrates the most basic SPIFFE/SPIRE setup:
// a single SPIRE server and a single SPIRE agent connected via join token attestation.
//
// This is the first learning scenario. It uses step.RunStep so each phase has:
//   - Explain text before action, tied to server.conf and agent.conf
//   - Observe text after action, so developers can inspect state and logs
//
// The flow teaches core SPIRE ideas:
//   - How trust_domain mirmat.org becomes the root of all SPIFFE IDs
//   - How the server initializes SQLite state, plugins, and signing authority
//   - How a join token bootstraps the first agent into the trust domain
//   - How the agent exposes the Workload API after successful attestation
//
// Usage:
//
//	scenario-01-simple [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
