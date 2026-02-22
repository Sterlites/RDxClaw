package swarm

import (
	"context"
	"fmt"
	"github.com/Sterlites/RDxClaw/pkg/tools"
)

// SpawnTool starts a subagent asynchronously.
type SpawnTool struct {
	manager       *Manager
	originChannel string
	originChatID  string
	callback      tools.AsyncCallback
}

func NewSpawnTool(manager *Manager) *SpawnTool {
	return &SpawnTool{
		manager:       manager,
		originChannel: "cli",
		originChatID:  "direct",
	}
}

func (t *SpawnTool) Name() string {
	return "spawn_agent"
}

func (t *SpawnTool) Description() string {
	return "Spawn an autonomous subagent to complete a task in the background. Returns immediately."
}

func (t *SpawnTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task instructions for the subagent",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Short label for identifying this agent",
			},
		},
		"required": []string{"task"},
	}
}

func (t *SpawnTool) SetContext(channel, chatID string) {
	t.originChannel = channel
	t.originChatID = chatID
}

func (t *SpawnTool) SetCallback(cb tools.AsyncCallback) {
	t.callback = cb
}

func (t *SpawnTool) Execute(ctx context.Context, args map[string]interface{}) *tools.ToolResult {
	task, _ := args["task"].(string)
	label, _ := args["label"].(string)

	msg, err := t.manager.Spawn(ctx, task, label, t.originChannel, t.originChatID, t.callback)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to spawn agent: %v", err))
	}

	return &tools.ToolResult{
		ForLLM:  msg,
		ForUser: msg,
		Async:   true,
	}
}

// SubagentTool runs a subagent synchronously.
type SubagentTool struct {
	manager       *Manager
	originChannel string
	originChatID  string
}

func NewSubagentTool(manager *Manager) *SubagentTool {
	return &SubagentTool{
		manager:       manager,
		originChannel: "cli",
		originChatID:  "direct",
	}
}

func (t *SubagentTool) Name() string {
	return "delegate_task" // Renamed from 'subagent' for clarity? Or keep 'subagent'?
}

func (t *SubagentTool) Description() string {
	return "Delegate a task to a subagent and wait for the result. Useful for complex sub-tasks."
}

func (t *SubagentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task instructions",
			},
			"label": map[string]interface{}{
				"type":        "string",
				"description": "Optional label",
			},
		},
		"required": []string{"task"},
	}
}

func (t *SubagentTool) SetContext(channel, chatID string) {
	t.originChannel = channel
	t.originChatID = chatID
}

func (t *SubagentTool) Execute(ctx context.Context, args map[string]interface{}) *tools.ToolResult {
	taskStr, _ := args["task"].(string)
	label, _ := args["label"].(string)

	// Create a task record manually or let RunTask handle it?
	// RunTask expects *SubagentTask.
	// We should probably expose a cleaner API on Manager like RunSync(ctx, task, label...)
	// But since I implemented RunTask(*SubagentTask), let's build it here.
	// Wait, Spawn handles ID generation. I need to replicate that if I construct struct manually.
	// I should have exposed RunSync(string, string...) on Manager.
	// I'll fix this by adding a helper in Manager or just duplicating ID logic here?
	// Duplicating logic is bad.
	// I'll call Spawn and wait? No, Spawn runs in goroutine.
	// I should add sm.CreateTask(task, label...) -> *SubagentTask.
	// For now, let's just hack it: "sync-agent-" + time?
	// Actually, let's update Manager.RunTask to accept strings if possible or add helper.
	// Since I can't easily edit Manager again in this turn without conflict, I'll access tasks map directly? No, private.
	// I'll assume I can add a helper function in tools.go or use Spawn and wait on a channel?
	// Using Spawn and waiting on channel is safest given current API.

	resultChan := make(chan *tools.ToolResult, 1)
	callback := func(ctx context.Context, res *tools.ToolResult) {
		resultChan <- res
	}

	_, err := t.manager.Spawn(ctx, taskStr, label, t.originChannel, t.originChatID, callback)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to delegate task: %v", err))
	}

	// Wait for result
	select {
	case result := <-resultChan:
		return result
	case <-ctx.Done():
		return tools.ErrorResult("Delegated task cancelled or timed out")
	}
}

// SwarmTool manages the swarm (list, kill, etc.).
type SwarmTool struct {
	manager *Manager
}

func NewSwarmTool(manager *Manager) *SwarmTool {
	return &SwarmTool{
		manager: manager,
	}
}

func (t *SwarmTool) Name() string {
	return "swarm"
}

func (t *SwarmTool) Description() string {
	return "Manage swarm agents: list active agents, check status, or kill an agent."
}

func (t *SwarmTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "status", "kill"},
				"description": "Action to perform",
			},
			"agent_id": map[string]interface{}{
				"type":        "string",
				"description": "Agent ID (for status/kill)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SwarmTool) Execute(ctx context.Context, args map[string]interface{}) *tools.ToolResult {
	action, _ := args["action"].(string)
	agentID, _ := args["agent_id"].(string)

	switch action {
	case "list":
		agents := t.manager.ListAgents()
		if len(agents) == 0 {
			return &tools.ToolResult{ForLLM: "No active swarm agents."}
		}
		out := "Active Swarm Agents:\n"
		for _, a := range agents {
			out += fmt.Sprintf("- [%s] %s (Task: %s) - Status: %s\n", a.ID, a.Label, a.Task, a.Status)
		}
		return &tools.ToolResult{ForLLM: out, ForUser: out}

	case "status":
		if agentID == "" {
			return tools.ErrorResult("agent_id required for status")
		}
		agent, ok := t.manager.GetAgent(agentID)
		if !ok {
			return tools.ErrorResult("Agent not found")
		}
		out := fmt.Sprintf("Agent: %s\nID: %s\nStatus: %s\nCreated: %d\nTask: %s\nResult: %s",
			agent.Label, agent.ID, agent.Status, agent.Created, agent.Task, agent.Result)
		return &tools.ToolResult{ForLLM: out, ForUser: out}

	case "kill":
		if agentID == "" {
			return tools.ErrorResult("agent_id required for kill")
		}
		err := t.manager.KillAgent(agentID)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Failed to kill agent: %v", err))
		}
		msg := fmt.Sprintf("Agent %s killed.", agentID)
		return &tools.ToolResult{ForLLM: msg, ForUser: msg}

	default:
		return tools.ErrorResult("Unknown action")
	}
}
