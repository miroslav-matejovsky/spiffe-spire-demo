// Command scenario-05-svid-api demonstrates service-to-service authentication
// using SPIFFE X.509 SVIDs. Two Go services (svid-server and svid-client)
// communicate over mTLS, with identities provided by the SPIRE Workload API.
//
// This scenario shows:
//   - How workloads connect to the Workload API to get SVIDs
//   - How mTLS is established using SPIFFE identities
//   - How workload registration maps processes to SPIFFE IDs (via UID selectors)
//   - How the trust bundle enables mutual verification
//
// Usage:
//
//	scenario-05-svid-api [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
