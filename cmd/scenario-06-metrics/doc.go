// Command scenario-06-metrics demonstrates SPIRE's telemetry integration
// with Prometheus and Graphite. It uses x509pop attestation and adds
// observability collectors to the stack.
//
// This scenario shows:
//   - How SPIRE emits metrics (StatsD format to Graphite)
//   - How Prometheus scrapes SPIRE server metrics
//   - How to monitor SPIRE health and performance in production
//
// Usage:
//
//	scenario-06-metrics [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
