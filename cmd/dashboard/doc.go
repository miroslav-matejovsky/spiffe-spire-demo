// Command dashboard serves a web UI for viewing SPIRE server state.
// It connects to the SPIRE server API via gRPC and displays agents, entries,
// and trust bundle information.
//
// Code layout:
//   - main.go: entry point, gRPC connection, HTTP routing, template setup
//   - handlers.go: HTTP handler methods for each page
//   - types.go: data types for template rendering
//   - helpers.go: SPIFFE ID formatting, time formatting, selector stringification
//
// The agents page shows each agent's selectors and lists associated workloads
// by matching registration entries whose parent ID equals the agent SPIFFE ID.
// Entries display the hint field when set, providing human-readable metadata
// about each workload registration.
//
// Environment variables:
//   - SPIRE_SERVER_SOCKET: gRPC address of SPIRE server (default: unix:///tmp/spire-server/private/api.sock)
//   - PORT: HTTP listen port (default: 8080)
package main
