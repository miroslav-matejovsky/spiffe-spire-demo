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
	maxAttempts = 15
	retryDelay  = 2 * time.Second
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
	for attempt := 1; attempt <= 20; attempt++ {
		output, _ := compose.Exec(container, spireBin, "agent", "list")
		allFound := true
		for _, p := range patterns {
			if !strings.Contains(output, p) {
				allFound = false
				break
			}
		}
		log.Detailf("attempt %d/20: all agents found=%v", attempt, allFound)
		if allFound {
			log.Ok("All agents attested successfully.")
			return output, nil
		}
		time.Sleep(3 * time.Second)
	}
	return "", fmt.Errorf("not all agents attested after 20 attempts (patterns: %v)", patterns)
}

// CreateEntry registers a workload entry with the SPIRE server.
func CreateEntry(compose *podman.Compose, container, spiffeID, parentID, selector string, log *logging.Logger) error {
	log.Infof("Registering workload: %s (selector: %s)", spiffeID, selector)
	output, err := compose.Exec(container, spireBin, "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", selector,
	)
	if err != nil {
		return fmt.Errorf("entry create failed: %w\n%s", err, output)
	}
	log.Ok(fmt.Sprintf("Registered %s", spiffeID))
	return nil
}

// GetAgentID extracts the agent SPIFFE ID from the agent list output.
// Looks for a pattern like spiffe://mirmat.org/spire/agent/join_token/<uuid>.
func GetAgentID(compose *podman.Compose, container string, log *logging.Logger) (string, error) {
	output, err := compose.Exec(container, spireBin, "agent", "list")
	if err != nil {
		return "", fmt.Errorf("agent list failed: %w", err)
	}
	re := regexp.MustCompile(`(spiffe://mirmat\.org/spire/agent/join_token/[a-f0-9-]+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return "", fmt.Errorf("no agent SPIFFE ID found in output:\n%s", output)
	}
	log.Detailf("agent ID: %s", matches[1])
	return matches[1], nil
}
