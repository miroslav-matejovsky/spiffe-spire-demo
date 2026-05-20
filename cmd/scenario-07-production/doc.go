// Command scenario-07-production demonstrates a production-like SPIRE deployment
// with mixed node attestation, multiple workload pools, and full observability.
//
// This capstone scenario now uses the RunStep teaching flow. Each step explains
// why the action matters before it runs, then tells the learner what changed and
// what to verify after success. That makes the scenario map better to real ops.
//
// Core ideas shown here:
//   - Agent-1: join_token attestation for simple bootstrap hosts
//   - Agent-2: x509pop attestation for stronger device-backed hosts
//   - Multiple workloads registered under different parent IDs
//   - Prometheus + Graphite telemetry from first SPIRE startup events
//   - Dashboard visibility across agents, entries, and trust bundle
//
// Usage:
//
//	scenario-07-production [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
