package spirectl

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/logging"
	"github.com/miroslav-matejovsky/spiffe-spire-demo/internal/podman"
)

const (
	spireBin    = "/opt/spire/bin/spire-server"
	maxAttempts = 30
	retryDelay  = 3 * time.Second
)

// tokenRegex matches the SPIRE token output format: "Token: <value>"
var tokenRegex = regexp.MustCompile(`Token:\s+(\S+)`)

// Healthcheck polls the SPIRE server container until it reports healthy.
// Returns an error if the server does not become ready within maxAttempts.
func Healthcheck(compose *podman.Compose, container string, log *logging.Logger) error {
	log.Info("Waiting for SPIRE server to become ready...")
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := compose.Exec(container, spireBin, "healthcheck")
		log.Detailf("attempt %d/%d: healthcheck err=%v", attempt, maxAttempts, err)
		if err == nil {
			log.Ok("SPIRE server is ready.")
			return nil
		}
		time.Sleep(retryDelay)
	}
	return fmt.Errorf("SPIRE server did not become ready after %d attempts", maxAttempts)
}

// GenerateToken creates a join token for the given SPIFFE ID.
// Returns the token string.
func GenerateToken(compose *podman.Compose, container, spiffeID string, log *logging.Logger) (string, error) {
	log.Info("Generating join token...")
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := compose.Exec(container, spireBin, "token", "generate", "-spiffeID", spiffeID)
		log.Detailf("attempt %d/%d: token output=%q err=%v", attempt, maxAttempts, output, err)
		if err == nil {
			matches := tokenRegex.FindStringSubmatch(output)
			if len(matches) >= 2 {
				log.Detail("join token acquired")
				return matches[1], nil
			}
		}
		time.Sleep(retryDelay)
	}
	return "", fmt.Errorf("failed to generate join token for %s after %d attempts", spiffeID, maxAttempts)
}

// WaitForAgent polls the agent list until an agent matching the given pattern appears.
// The pattern is checked as a substring of the agent list output.
// Returns the full agent list output on success.
func WaitForAgent(compose *podman.Compose, container, pattern string, log *logging.Logger) (string, error) {
	log.Info("Waiting for agent attestation...")
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, _ := compose.Exec(container, spireBin, "agent", "list")
		log.Detailf("attempt %d/%d: agent list output:\n%s", attempt, maxAttempts, strings.TrimSpace(output))
		if strings.Contains(output, pattern) {
			log.Ok("Agent attested successfully.")
			return output, nil
		}
		time.Sleep(retryDelay)
	}
	return "", fmt.Errorf("agent with pattern %q not found after %d attempts", pattern, maxAttempts)
}

// WaitForAgents waits until all specified patterns appear in agent list output.
func WaitForAgents(compose *podman.Compose, container string, patterns []string, log *logging.Logger) (string, error) {
	log.Info("Waiting for all agents to attest...")
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, _ := compose.Exec(container, spireBin, "agent", "list")
		allFound := true
		for _, p := range patterns {
			if !strings.Contains(output, p) {
				allFound = false
				break
			}
		}
		log.Detailf("attempt %d/%d: all agents found=%v", attempt, maxAttempts, allFound)
		if allFound {
			log.Ok("All agents attested successfully.")
			return output, nil
		}
		time.Sleep(retryDelay)
	}
	return "", fmt.Errorf("not all agents attested after %d attempts (patterns: %v)", maxAttempts, patterns)
}

// CreateEntry registers a workload entry with the SPIRE server.
// Optional hint provides a human-readable description for the entry.
func CreateEntry(compose *podman.Compose, container, spiffeID, parentID, selector string, log *logging.Logger, hints ...string) error {
	log.Infof("Registering workload: %s (selector: %s)", spiffeID, selector)
	args := []string{"entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", selector,
	}
	if len(hints) > 0 && hints[0] != "" {
		args = append(args, "-hint", hints[0])
	}
	output, err := compose.Exec(container, append([]string{spireBin}, args...)...)
	if err != nil {
		return fmt.Errorf("entry create failed: %w\n%s", err, output)
	}
	log.Ok(fmt.Sprintf("Registered %s", spiffeID))
	return nil
}

// GetAgentIDByType extracts the agent SPIFFE ID for a given attestation type.
// agentType is the path segment used by SPIRE (e.g. "join_token" or "x509pop").
func GetAgentIDByType(compose *podman.Compose, container, agentType string, log *logging.Logger) (string, error) {
	output, err := compose.Exec(container, spireBin, "agent", "list")
	if err != nil {
		return "", fmt.Errorf("agent list failed: %w", err)
	}
	re := regexp.MustCompile(`(spiffe://mirmat\.org/spire/agent/` + regexp.QuoteMeta(agentType) + `/[^\s]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", fmt.Errorf("no %s agent SPIFFE ID found in output:\n%s", agentType, output)
	}
	log.Detailf("%s agent ID: %s", agentType, matches[1])
	return matches[1], nil
}

// GetAgentID extracts the agent SPIFFE ID from the agent list output.
// Looks for a pattern like spiffe://mirmat.org/spire/agent/join_token/<uuid>.
func GetAgentID(compose *podman.Compose, container string, log *logging.Logger) (string, error) {
	return GetAgentIDByType(compose, container, "join_token", log)
}
