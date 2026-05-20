package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	agent "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	bundle "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	entry "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
)

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

	// Fetch entries to find workloads associated with each agent.
	entryResp, _ := d.entries.ListEntries(ctx, &entry.ListEntriesRequest{})
	entryByParent := make(map[string][]entryRow)
	if entryResp != nil {
		for _, e := range entryResp.Entries {
			parentID := spiffeIDStr(e.ParentId)
			entryByParent[parentID] = append(entryByParent[parentID], entryRow{
				ID:        e.Id,
				SpiffeID:  spiffeIDStr(e.SpiffeId),
				ParentID:  parentID,
				Selectors: selectorsStr(e.Selectors),
				TTL:       e.X509SvidTtl,
				DnsNames:  strings.Join(e.DnsNames, ", "),
				Hint:      e.Hint,
				CreatedAt: formatUnixTime(e.CreatedAt),
			})
		}
	}

	rows := make([]agentRow, 0, len(resp.Agents))
	for _, a := range resp.Agents {
		agentID := spiffeIDStr(a.Id)
		rows = append(rows, agentRow{
			SpiffeID:    agentID,
			Attestation: a.AttestationType,
			ExpiresAt:   formatUnixTime(a.X509SvidExpiresAt),
			Banned:      a.Banned,
			CanReattest: a.CanReattest,
			Selectors:   selectorsStr(a.Selectors),
			Workloads:   entryByParent[agentID],
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
