// Command scenario-07-production demonstrates a production-like SPIRE deployment
// with multiple agents, attestation methods, workloads, and full observability.
//
// This scenario combines everything from previous scenarios:
//   - Agent-1: join_token attestation (simple bootstrap)
//   - Agent-2: x509pop attestation (certificate-based, like TPM)
//   - Multiple workloads registered across agents
//   - Prometheus + Graphite for metrics collection
//   - Dashboard for SPIRE API monitoring
//
// Usage:
//
//	scenario-07-production [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
