package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const (
	socketPath = "unix:///opt/spire/sockets/workload_api.sock"
)

func main() {
	ctx := context.Background()

	// Create X509Source — this connects to the Workload API and
	// automatically fetches and rotates X.509 SVIDs
	x509Source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer x509Source.Close()

	// Get our own SPIFFE ID for logging
	svid, err := x509Source.GetX509SVID()
	if err != nil {
		log.Fatalf("Unable to get X509-SVID: %v", err)
	}
	log.Printf("Server SPIFFE ID: %s", svid.ID)

	// Define the trust domain for authorization
	td, err := spiffeid.TrustDomainFromString("mirmat.org")
	if err != nil {
		log.Fatalf("Unable to parse trust domain: %v", err)
	}

	// Create TLS config that:
	// 1. Uses our SVID as the server certificate
	// 2. Requires client certificates (mTLS)
	// 3. Authorizes any client within our trust domain
	tlsConfig := tlsconfig.MTLSServerConfig(x509Source, x509Source, tlsconfig.AuthorizeMemberOf(td))

	// Set up HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Extract client SPIFFE ID from the mTLS connection
		clientID := "unknown"
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 && len(r.TLS.PeerCertificates[0].URIs) > 0 {
			if id, err := spiffeid.FromURI(r.TLS.PeerCertificates[0].URIs[0]); err == nil {
				clientID = id.String()
			}
		}

		log.Printf("Request from client: %s", clientID)
		fmt.Fprintf(w, "Hello from server!\n\nServer identity: %s\nClient identity: %s\n", svid.ID, clientID)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "healthy")
	})

	// Create HTTPS server with SPIFFE-based TLS
	port := os.Getenv("PORT")
	if port == "" {
		port = "8443"
	}
	server := &http.Server{
		Addr:      ":" + port,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	log.Printf("Server listening on :%s with mTLS (SPIFFE)", port)
	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
