package swarm

import (
	"context"
	"testing"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/Sterlites/RDxClaw/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// PanicTool always panics
type PanicTool struct{}

func (t *PanicTool) Name() string { return "panic_tool" }
func (t *PanicTool) Description() string { return "A tool that panics for testing" }
func (t *PanicTool) Parameters() map[string]any { return map[string]any{"type": "object", "properties": map[string]any{}} }
func (t *PanicTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	panic("test panic")
}

func TestManager_PanicRecovery(t *testing.T) {
	msgBus := bus.NewMessageBus()
	
	// Create a provider that panics
	provider := &panicProvider{}
	manager := NewManager(provider, "test-model", "/tmp", msgBus)
	
	registry := tools.NewToolRegistry()
	manager.SetToolRegistry(registry)

	ctx := context.Background()
	_, err := manager.Spawn(ctx, "Trigger panic", "panic-agent", "system", "test", nil)
	assert.NoError(t, err)

	// Wait for agent to fail due to panic
	var finalStatus string
	var finalResult string
	for i := 0; i < 20; i++ {
		agents := manager.ListAgents()
		if len(agents) > 0 && agents[0].Status == "failed" {
			finalStatus = "failed"
			finalResult = agents[0].Result
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	assert.Equal(t, "failed", finalStatus, "Agent should have failed due to panic")
	assert.Contains(t, finalResult, "Panic: provider panic", "Result should contain the panic message")
}

type panicProvider struct {
}

func (p *panicProvider) Chat(ctx context.Context, messages []providers.Message, toolDefs []providers.ToolDefinition, model string, options map[string]any) (*providers.LLMResponse, error) {
	panic("provider panic")
}

func (p *panicProvider) GetDefaultModel() string { return "test" }
func (p *panicProvider) EstimateTokens(messages []providers.Message) int { return 0 }
