package agentconfig

import (
	"context"
	"reflect"
	"testing"
)

func TestParseAgentsNormalizesAndDeduplicates(t *testing.T) {
	got, err := ParseAgents(" codex,claude-code,codex, opencode ")
	want := []Agent{AgentCodex, AgentClaudeCode, AgentOpenCode}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAgents()=%v, %v", got, err)
	}
	for _, value := range []string{"", "codex,", "unknown", "codex,,opencode"} {
		if _, err = ParseAgents(value); err == nil {
			t.Fatalf("ParseAgents(%q) succeeded", value)
		}
	}
}

type configuratorStub struct{ agents []Agent }

func (stub *configuratorStub) Configure(_ context.Context, agent Agent, path string) Result {
	stub.agents = append(stub.agents, agent)
	if agent == AgentClaudeCode {
		return Result{Agent: agent, Status: StatusWarning, SharedMemoryPath: path}
	}
	return Result{Agent: agent, Status: StatusConfigured, SharedMemoryPath: path}
}

func TestServiceContinuesAfterWarning(t *testing.T) {
	stub := &configuratorStub{}
	results := NewService(stub).Configure(context.Background(), []Agent{AgentClaudeCode, AgentCodex}, "/shared")
	if len(results) != 2 || results[0].Status != StatusWarning || results[1].Status != StatusConfigured {
		t.Fatalf("results=%#v", results)
	}
}
