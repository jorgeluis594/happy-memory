// Package agentconfig coordinates persistent filesystem access for coding agents.
package agentconfig

import (
	"context"
	"errors"
	"strings"
)

// Agent identifies one supported coding agent.
type Agent string

const (
	// AgentCodex identifies OpenAI Codex.
	AgentCodex Agent = "codex"
	// AgentClaudeCode identifies Anthropic Claude Code.
	AgentClaudeCode Agent = "claude-code"
	// AgentOpenCode identifies OpenCode.
	AgentOpenCode Agent = "opencode"

	// StatusConfigured reports a newly applied configuration.
	StatusConfigured = "configured"
	// StatusAlreadyConfigured reports an idempotent no-op.
	StatusAlreadyConfigured = "already_configured"
	// StatusWarning reports a non-fatal configuration failure.
	StatusWarning = "warning"
)

// Warning describes a non-fatal agent configuration failure.
type Warning struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

// Result reports the outcome for one requested agent.
type Result struct {
	Agent            Agent    `json:"agent"`
	Status           string   `json:"status"`
	ConfigPath       string   `json:"config_path"`
	SharedMemoryPath string   `json:"shared_memory_path"`
	Warning          *Warning `json:"warning,omitempty"`
}

// Configurator applies one agent-specific configuration change.
type Configurator interface {
	Configure(context.Context, Agent, string) Result
}

// Service coordinates independent configuration attempts in request order.
type Service struct{ configurator Configurator }

// NewService creates an agent configuration service.
func NewService(configurator Configurator) *Service { return &Service{configurator: configurator} }

// Configure attempts every agent even when an earlier attempt returns a warning.
func (service *Service) Configure(ctx context.Context, agents []Agent, sharedMemoryPath string) []Result {
	results := make([]Result, 0, len(agents))
	for _, agent := range agents {
		results = append(results, service.configurator.Configure(ctx, agent, sharedMemoryPath))
	}
	return results
}

// ParseAgents validates a comma-separated list, trims whitespace, and removes duplicates.
func ParseAgents(value string) ([]Agent, error) {
	parts := strings.Split(value, ",")
	agents := make([]Agent, 0, len(parts))
	seen := make(map[Agent]struct{}, len(parts))
	for _, part := range parts {
		agent := Agent(strings.TrimSpace(part))
		if !agent.valid() {
			return nil, errors.New("invalid agent")
		}
		if _, found := seen[agent]; found {
			continue
		}
		seen[agent] = struct{}{}
		agents = append(agents, agent)
	}
	return agents, nil
}

func (agent Agent) valid() bool {
	return agent == AgentCodex || agent == AgentClaudeCode || agent == AgentOpenCode
}
