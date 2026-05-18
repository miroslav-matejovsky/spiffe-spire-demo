package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const (
	socketPath = "unix:///opt/spire/sockets/workload_api.sock"
)

func main() {
	ctx := context.Background()

	// Create X509Source for fetching and rotating SVIDs
	x509Source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer x509Source.Close()

	svid, err := x509Source.GetX509SVID()
	if err != nil {
		log.Fatalf("Unable to get X509-SVID: %v", err)
	}
	log.Printf("Client SPIFFE ID: %s", svid.ID)

	td, err := spiffeid.TrustDomainFromString("mirmat.org")
	if err != nil {
		log.Fatalf("Unable to parse trust domain: %v", err)
	}

	// Create mTLS client config — uses our SVID and validates server is in our trust domain
	tlsConfig := tlsconfig.MTLSClientConfig(x509Source, x509Source, tlsconfig.AuthorizeMemberOf(td))
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}

	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = "https://svid-server:8443"
	}

	// Call the server in a loop to demonstrate ongoing mTLS communication
	for i := 1; ; i++ {
		log.Printf("--- Request #%d ---", i)
		if err := doRequest(client, serverURL); err != nil {
			log.Printf("%v", err)
		}
		if i >= 5 {
			log.Println("Completed 5 requests. Client done.")
			break
		}
		time.Sleep(10 * time.Second)
	}
}

func doRequest(client *http.Client, serverURL string) error {
	resp, err := client.Get(serverURL)
	if err != nil {
		return fmt.Errorf("error calling server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	fmt.Printf("Response (status %d):\n%s\n", resp.StatusCode, string(body))
	return nil
}
