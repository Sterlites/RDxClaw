package swarm

import (
	"context"
	"testing"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/stretchr/testify/assert"
)

// MockProvider satisfies providers.LLMProvider for testing
type MockProvider struct {
	Response string
}

func (m *MockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content: m.Response,
		Usage: providers.UsageInfo{
			TotalTokens: 100,
		},
	}, nil
}

func (m *MockProvider) EstimateTokens(messages []providers.Message) int {
	return 100
}

func TestManager_Lifecycle(t *testing.T) {
	msgBus := bus.NewMessageBus()
	provider := &MockProvider{Response: "Task complete."}
	manager := NewManager(provider, "test-model", "/tmp", msgBus)

	ctx := context.Background()
	
	// Test Spawn
	msg, err := manager.Spawn(ctx, "Test task", "test-agent", "test-channel", "test-chat", nil)
	assert.NoError(t, err)
	assert.Contains(t, msg, "Spawned agent 'test-agent'")

	agents := manager.ListAgents()
	assert.Len(t, agents, 1)
	agentID := agents[0].ID
	assert.Equal(t, "running", agents[0].Status)
	assert.Equal(t, "test-agent", agents[0].Label)

	// Wait for agent to finish (it runs in background)
	// Since MockProvider returns immediately, it should finish quickly
	maxWait := 10
	for i := 0; i < maxWait; i++ {
		agent, _ := manager.GetAgent(agentID)
		if agent.Status == "completed" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	agent, ok := manager.GetAgent(agentID)
	assert.True(t, ok)
	assert.Equal(t, "completed", agent.Status)
	assert.Equal(t, "Task complete.", agent.Result)
	assert.True(t, agent.Finished > 0)
}

func TestManager_Kill(t *testing.T) {
	msgBus := bus.NewMessageBus()
	// Slow provider to simulate long running task
	provider := &MockProvider{Response: "Done"}
	manager := NewManager(provider, "test-model", "/tmp", msgBus)

	// We can't easily wait for it to be mid-execution with a simple mock without channels
	// but we can test the status transition.
	
	id, _ := manager.Spawn(context.Background(), "Long task", "kill-me", "ch", "chat", nil)
	// Extract ID from message: "Spawned agent 'kill-me' (ID: agent-1) for task: Long task"
	// ID is generated as agent-1, agent-2...
	agentID := "agent-1" 

	err := manager.KillAgent(agentID)
	assert.NoError(t, err)

	agent, _ := manager.GetAgent(agentID)
	assert.Equal(t, "cancelled", agent.Status)
}
