// Command scenario-06-metrics demonstrates SPIRE telemetry with
// Prometheus, StatsD, and Graphite. It uses x509pop attestation and adds
// observability collectors to stack.
//
// This scenario shows:
//   - How telemetry {} blocks enable pull and push metrics paths in SPIRE
//   - How Prometheus scrapes SPIRE server and agent at path "/"
//   - How StatsD pushes counters and gauges into Graphite for browsing
//   - How to compare SPIRE API state in dashboard with telemetry views
//
// Usage:
//
//	scenario-06-metrics [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
