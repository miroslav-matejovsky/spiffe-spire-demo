// Command dashboard serves a web UI for viewing SPIRE server state.
// It connects to the SPIRE server API via gRPC and displays agents, entries,
// and trust bundle information.
//
// Environment variables:
//   - SPIRE_SERVER_SOCKET: gRPC address of SPIRE server (default: unix:///tmp/spire-server/private/api.sock)
//   - PORT: HTTP listen port (default: 8080)
package main
