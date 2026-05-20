package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	agent "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	bundle "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	entry "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed templates/*
var templateFS embed.FS

// dashboard holds gRPC clients and parsed page templates.
type dashboard struct {
	agents  agent.AgentClient
	entries entry.EntryClient
	bundles bundle.BundleClient
	pages   map[string]*template.Template
}

func newDashboard(conn *grpc.ClientConn) *dashboard {
	funcMap := template.FuncMap{
		"now": func() string { return time.Now().Format("15:04:05") },
	}

	parsePage := func(page string) *template.Template {
		return template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/"+page),
		)
	}

	return &dashboard{
		agents:  agent.NewAgentClient(conn),
		entries: entry.NewEntryClient(conn),
		bundles: bundle.NewBundleClient(conn),
		pages: map[string]*template.Template{
			"index":   parsePage("index.html"),
			"agents":  parsePage("agents.html"),
			"entries": parsePage("entries.html"),
			"bundles": parsePage("bundles.html"),
		},
	}
}

func main() {
	socketPath := os.Getenv("SPIRE_SERVER_SOCKET")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-server/private/api.sock"
	}

	conn, err := grpc.NewClient(socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect to SPIRE server: %v", err)
	}
	defer conn.Close()

	d := newDashboard(conn)

	mux := http.NewServeMux()
	mux.HandleFunc("/", d.handleIndex)
	mux.HandleFunc("/agents", d.handleAgents)
	mux.HandleFunc("/entries", d.handleEntries)
	mux.HandleFunc("/bundles", d.handleBundles)
	mux.HandleFunc("/health", d.handleHealth)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("SPIRE Dashboard listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
