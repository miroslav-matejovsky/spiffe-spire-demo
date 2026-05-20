// Package spirectl provides helpers for interacting with the SPIRE server CLI
// running inside containers. It wraps common SPIRE administration operations that
// scenarios need during startup.
//
// Operations provided:
//   - Healthcheck: poll the SPIRE server until it responds healthy
//   - GenerateToken: create a join token for agent attestation
//   - WaitForAgent: poll agent list until an agent with matching pattern appears
//   - WaitForAgents: poll until multiple attestation types are all present
//   - CreateEntry: register a workload entry (SPIFFE ID + selector + optional hint)
//   - GetAgentID / GetAgentIDByType: extract agent SPIFFE ID from agent list output
//
// All operations use podman-compose exec to run spire-server CLI commands inside
// the running SPIRE server container. They include retry logic with configurable
// attempts and delays, matching the behavior of the original PowerShell scripts.
package spirectl
