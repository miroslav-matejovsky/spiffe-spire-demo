package main

// pageData is the top-level data passed to every HTML template.
type pageData struct {
	Title   string
	Active  string
	Content interface{}
	Error   string
	Time    string
}

// overviewData holds summary stats shown on the index page.
type overviewData struct {
	Healthy     bool
	AgentCount  int
	EntryCount  int
	TrustDomain string
}

// agentRow represents one agent in the agents table.
type agentRow struct {
	SpiffeID    string
	Attestation string
	ExpiresAt   string
	Banned      bool
	CanReattest bool
	Selectors   string
	Workloads   []entryRow
}

// entryRow represents one registration entry in the entries table.
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

// bundleData holds trust bundle information for the bundles page.
type bundleData struct {
	TrustDomain    string
	X509Count      int
	JwtCount       int
	RefreshHint    int64
	SequenceNumber uint64
}
