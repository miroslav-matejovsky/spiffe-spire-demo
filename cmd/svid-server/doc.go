// Command svid-server is an HTTPS server that uses SPIFFE X.509 SVIDs for mTLS.
// It connects to the SPIRE Workload API to fetch its identity and validates
// incoming client connections against the trust domain.
//
// This binary runs inside a container as UID 10001, which maps to the registered
// SPIFFE ID spiffe://mirmat.org/svid-server.
//
// Environment variables:
//   - PORT: HTTPS listen port (default: 8443)
package main
