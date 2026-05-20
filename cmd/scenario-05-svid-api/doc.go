// Command scenario-05-svid-api demonstrates service-to-service authentication
// using SPIFFE X.509 SVIDs. Two Go services (svid-server and svid-client)
// communicate over mTLS, with identities provided by the SPIRE Workload API.
//
// This scenario shows:
//   - How workloads use go-spiffe X509Source to fetch and rotate SVIDs
//   - How tlsconfig builds mTLS client and server config from Workload API data
//   - How workload registration maps processes to SPIFFE IDs (via UID selectors)
//   - How the trust bundle enables mutual verification and SPIFFE-based auth
//   - How RunStep turns startup into a guided, inspectable learning flow
//
// Usage:
//
//	scenario-05-svid-api [flags]
//	  --step     Pause between steps for interactive learning
//	  --verbose  Show detailed debug output
//	  --down     Tear down the scenario instead of starting it
package main
