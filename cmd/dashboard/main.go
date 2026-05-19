package main

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	agent "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	bundle "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	entry "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:embed templates/*
var templateFS embed.FS

type pageData struct {
	Title   string
	Active  string
	Content interface{}
	Error   string
	Time    string
}

type overviewData struct {
	Healthy     bool
	AgentCount  int
	EntryCount  int
	TrustDomain string
}

type agentRow struct {
	SpiffeID    string
	Attestation string
	ExpiresAt   string
	Banned      bool
	CanReattest bool
}

type entryRow struct {
	ID        string
	SpiffeID  string
	ParentID  string
	Selectors string
	TTL       int32
	DnsNames  string
	Hint      string
	CreatedAt string
}

type bundleData struct {
	TrustDomain    string
	X509Count      int
	JwtCount       int
	RefreshHint    int64
	SequenceNumber uint64
}

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

func (d *dashboard) render(w http.ResponseWriter, page string, data pageData) {
	data.Time = time.Now().Format("15:04:05")
	tmpl, ok := d.pages[page]
	if !ok {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func (d *dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data := pageData{Title: "Overview", Active: "overview"}
	overview := overviewData{Healthy: true}
	var errors []string

	agentResp, err := d.agents.ListAgents(ctx, &agent.ListAgentsRequest{})
	if err != nil {
		overview.Healthy = false
		errors = append(errors, fmt.Sprintf("agents: %v", err))
	} else {
		overview.AgentCount = len(agentResp.Agents)
	}

	entryResp, err := d.entries.ListEntries(ctx, &entry.ListEntriesRequest{})
	if err != nil {
		overview.Healthy = false
		errors = append(errors, fmt.Sprintf("entries: %v", err))
	} else {
		overview.EntryCount = len(entryResp.Entries)
	}

	bundleResp, err := d.bundles.GetBundle(ctx, &bundle.GetBundleRequest{})
	if err != nil {
		overview.Healthy = false
		errors = append(errors, fmt.Sprintf("bundle: %v", err))
	} else {
		overview.TrustDomain = bundleResp.TrustDomain
	}

	if len(errors) > 0 {
		data.Error = strings.Join(errors, "\n")
	}
	data.Content = overview
	d.render(w, "index", data)
}

func (d *dashboard) handleAgents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data := pageData{Title: "Agents", Active: "agents"}

	resp, err := d.agents.ListAgents(ctx, &agent.ListAgentsRequest{})
	if err != nil {
		data.Error = fmt.Sprintf("Failed to list agents: %v", err)
		d.render(w, "agents", data)
		return
	}

	rows := make([]agentRow, 0, len(resp.Agents))
	for _, a := range resp.Agents {
		rows = append(rows, agentRow{
			SpiffeID:    spiffeIDStr(a.Id),
			Attestation: a.AttestationType,
			ExpiresAt:   formatUnixTime(a.X509SvidExpiresAt),
			Banned:      a.Banned,
			CanReattest: a.CanReattest,
		})
	}
	data.Content = rows
	d.render(w, "agents", data)
}

func (d *dashboard) handleEntries(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data := pageData{Title: "Entries", Active: "entries"}

	resp, err := d.entries.ListEntries(ctx, &entry.ListEntriesRequest{})
	if err != nil {
		data.Error = fmt.Sprintf("Failed to list entries: %v", err)
		d.render(w, "entries", data)
		return
	}

	rows := make([]entryRow, 0, len(resp.Entries))
	for _, e := range resp.Entries {
		rows = append(rows, entryRow{
			ID:        e.Id,
			SpiffeID:  spiffeIDStr(e.SpiffeId),
			ParentID:  spiffeIDStr(e.ParentId),
			Selectors: selectorsStr(e.Selectors),
			TTL:       e.X509SvidTtl,
			DnsNames:  strings.Join(e.DnsNames, ", "),
			Hint:      e.Hint,
			CreatedAt: formatUnixTime(e.CreatedAt),
		})
	}
	data.Content = rows
	d.render(w, "entries", data)
}

func (d *dashboard) handleBundles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	data := pageData{Title: "Trust Bundle", Active: "bundles"}

	resp, err := d.bundles.GetBundle(ctx, &bundle.GetBundleRequest{})
	if err != nil {
		data.Error = fmt.Sprintf("Failed to get bundle: %v", err)
		d.render(w, "bundles", data)
		return
	}

	data.Content = bundleData{
		TrustDomain:    resp.TrustDomain,
		X509Count:      len(resp.X509Authorities),
		JwtCount:       len(resp.JwtAuthorities),
		RefreshHint:    resp.RefreshHint,
		SequenceNumber: resp.SequenceNumber,
	}
	d.render(w, "bundles", data)
}

func (d *dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	_, err := d.bundles.GetBundle(ctx, &bundle.GetBundleRequest{})
	if err != nil {
		http.Error(w, "SPIRE server unreachable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func spiffeIDStr(id *apitypes.SPIFFEID) string {
	if id == nil {
		return ""
	}
	return fmt.Sprintf("spiffe://%s%s", id.TrustDomain, id.Path)
}

func selectorsStr(sels []*apitypes.Selector) string {
	parts := make([]string, 0, len(sels))
	for _, s := range sels {
		parts = append(parts, s.Type+":"+s.Value)
	}
	return strings.Join(parts, ", ")
}

func formatUnixTime(ts int64) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05 UTC")
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
