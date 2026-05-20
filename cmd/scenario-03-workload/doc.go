// Command scenario-03-workload demonstrates workload identity - core SPIFFE
// value for applications. It walks through how registration policy, Workload
// API socket sharing, PID namespace sharing for unix attestation, and short-
// lived SVID rotation fit together to give running workloads an identity.
//
// Each phase uses RunStep so learner sees explanation before action and
// observation after action. Scenario starts stack, then leaves two key manual
// follow-ups:
//
//	register   Create workload registration entry on SPIRE server
//	fetch      Read workload logs and inspect streamed X.509-SVID data
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
