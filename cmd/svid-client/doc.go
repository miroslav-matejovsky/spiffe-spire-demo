// Command svid-client is an HTTPS client that uses SPIFFE X.509 SVIDs for mTLS.
// It connects to the SPIRE Workload API to fetch its identity and calls the
// svid-server using mutual TLS authentication.
//
// This binary runs inside a container as UID 10002, which maps to the registered
// SPIFFE ID spiffe://mirmat.org/svid-client.
//
// Environment variables:
//   - SERVER_URL: URL of the svid-server (default: https://svid-server:8443)
package main
