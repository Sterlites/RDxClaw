package swarm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/Sterlites/RDxClaw/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// MockProvider satisfies providers.LLMProvider for testing
type MockProvider struct {
	Response string
}

func (m *MockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content: m.Response,
		Usage: &providers.UsageInfo{
			TotalTokens: 100,
		},
	}, nil
}

func (m *MockProvider) GetDefaultModel() string {
	return "test-model"
}

func (m *MockProvider) EstimateTokens(messages []providers.Message) int {
	return 100
}

func TestManager_Lifecycle(t *testing.T) {
	msgBus := bus.NewMessageBus()
	provider := &MockProvider{Response: "Task complete."}
	manager := NewManager(provider, "test-model", t.TempDir(), msgBus)

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
	manager := NewManager(provider, "test-model", t.TempDir(), msgBus)

	// We can't easily wait for it to be mid-execution with a simple mock without channels
	// but we can test the status transition.

	_, _ = manager.Spawn(context.Background(), "Long task", "kill-me", "ch", "chat", nil)
	// Extract ID from message: "Spawned agent 'kill-me' (ID: agent-1) for task: Long task"
	// ID is generated as agent-1, agent-2...
	agentID := "agent-1"

	err := manager.KillAgent(agentID)
	assert.NoError(t, err)

	agent, _ := manager.GetAgent(agentID)
	assert.Equal(t, "cancelled", agent.Status)
}

// FailingMockProvider always returns an error (simulates 504 Gateway Timeout)
type FailingMockProvider struct{}

func (m *FailingMockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	return nil, fmt.Errorf("API request failed: Status: 504")
}

func (m *FailingMockProvider) GetDefaultModel() string {
	return "test-model"
}

func (m *FailingMockProvider) EstimateTokens(messages []providers.Message) int {
	return 100
}

// TestManager_SpawnWithError verifies that when RunToolLoop returns an error (nil result),
// the Spawn goroutine doesn't panic and correctly marks the task as failed.
func TestManager_SpawnWithError(t *testing.T) {
	msgBus := bus.NewMessageBus()
	provider := &FailingMockProvider{}
	manager := NewManager(provider, "test-model", t.TempDir(), msgBus)

	ctx := context.Background()

	callbackCalled := make(chan *tools.ToolResult, 1)
	callback := func(ctx context.Context, result *tools.ToolResult) {
		callbackCalled <- result
	}

	msg, err := manager.Spawn(ctx, "Test failing task", "fail-agent", "test-channel", "test-chat", callback)
	assert.NoError(t, err)
	assert.Contains(t, msg, "Spawned agent 'fail-agent'")

	// Wait for agent to finish
	maxWait := 10
	agentID := "agent-1"
	for i := 0; i < maxWait; i++ {
		agent, _ := manager.GetAgent(agentID)
		if agent.Status != "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify task is marked as failed, not crashed
	agent, ok := manager.GetAgent(agentID)
	assert.True(t, ok)
	assert.Equal(t, "failed", agent.Status)
	assert.Contains(t, agent.Result, "Error:")
	assert.True(t, agent.Finished > 0)

	// Verify callback was called with error
	select {
	case result := <-callbackCalled:
		assert.True(t, result.IsError)
		assert.NotNil(t, result.Err)
		assert.Contains(t, result.ForLLM, "Agent failed")
	case <-time.After(2 * time.Second):
		t.Fatal("Callback was not called within timeout")
	}
}

// PanickingMockProvider panics during Chat (simulates unexpected crash)
type PanickingMockProvider struct{}

func (m *PanickingMockProvider) Chat(ctx context.Context, messages []providers.Message, tools []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	panic("unexpected nil pointer in provider")
}

func (m *PanickingMockProvider) GetDefaultModel() string {
	return "test-model"
}

func (m *PanickingMockProvider) EstimateTokens(messages []providers.Message) int {
	return 100
}

// TestManager_PanicRecovery verifies that a panic during task execution is
// caught and the task is marked as failed instead of crashing the server.
func TestManager_PanicRecovery(t *testing.T) {
	msgBus := bus.NewMessageBus()
	provider := &PanickingMockProvider{}
	manager := NewManager(provider, "test-model", t.TempDir(), msgBus)

	ctx := context.Background()

	callbackCalled := make(chan *tools.ToolResult, 1)
	callback := func(ctx context.Context, result *tools.ToolResult) {
		callbackCalled <- result
	}

	msg, err := manager.Spawn(ctx, "Panicking task", "panic-agent", "test-channel", "test-chat", callback)
	assert.NoError(t, err)
	assert.Contains(t, msg, "Spawned agent 'panic-agent'")

	agentID := "agent-1"

	// Wait for agent to finish (should recover from panic, not crash)
	maxWait := 10
	for i := 0; i < maxWait; i++ {
		agent, _ := manager.GetAgent(agentID)
		if agent.Status != "running" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify task is marked as failed, NOT that the process crashed
	agent, ok := manager.GetAgent(agentID)
	assert.True(t, ok)
	assert.Equal(t, "failed", agent.Status)
	assert.Contains(t, agent.Result, "panic")
	assert.True(t, agent.Finished > 0)

	// Verify callback was called with error
	select {
	case result := <-callbackCalled:
		assert.True(t, result.IsError)
	case <-time.After(2 * time.Second):
		t.Fatal("Callback was not called within timeout after panic recovery")
	}

	// Verify the manager is still functional (can spawn more agents)
	msg2, err2 := manager.Spawn(ctx, "Post-panic task", "survivor", "ch", "chat", nil)
	assert.NoError(t, err2)
	assert.Contains(t, msg2, "Spawned agent 'survivor'")
}
