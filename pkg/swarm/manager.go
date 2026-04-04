package swarm

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/Sterlites/RDxClaw/pkg/tools"
)

type SubagentTask struct {
	ID            string `json:"id"`
	Task          string `json:"task"`
	Label         string `json:"label"`
	OriginChannel string `json:"origin_channel"`
	OriginChatID  string `json:"origin_chat_id"`
	Status        string `json:"status"` // running, completed, failed, cancelled
	Result        string `json:"result,omitempty"`
	TokensUsed    int    `json:"tokens_used,omitempty"`
	Created       int64  `json:"created"`
	Finished      int64  `json:"finished,omitempty"`
	cancel        context.CancelFunc
}

// Manager coordinates swarm agents.
type Manager struct {
	tasks         map[string]*SubagentTask
	mu            sync.RWMutex
	provider      providers.LLMProvider
	defaultModel  string
	bus           *bus.MessageBus
	workspace     string
	registry      *tools.ToolRegistry
	maxIterations int
	persistence   *Persistence
	nextID        int
}

// NewManager creates a new swarm manager.
func NewManager(provider providers.LLMProvider, defaultModel, workspace string, bus *bus.MessageBus) *Manager {
	persistence := NewPersistence(workspace)
	m := &Manager{
		tasks:         make(map[string]*SubagentTask),
		provider:      provider,
		defaultModel:  defaultModel,
		bus:           bus,
		workspace:     workspace,
		registry:      tools.NewToolRegistry(),
		maxIterations: 10,
		persistence:   persistence,
		nextID:        1,
	}
	
	// Try to recover any existing tasks
	if recovered, err := persistence.LoadTasks(); err == nil {
		m.tasks = recovered
		// Find highest ID to avoid collisions
		maxID := 0
		for idStr := range recovered {
			var id int
			if _, err := fmt.Sscanf(idStr, "agent-%d", &id); err == nil {
				if id > maxID {
					maxID = id
				}
			}
		}
		m.nextID = maxID + 1
	}

	return m
}

// save is a helper to persist tasks to disk. Must be called with mu held (or managed internally)
func (sm *Manager) save() {
	if err := sm.persistence.SaveTasks(sm.tasks); err != nil {
		fmt.Printf("[ERROR] swarm: failed to save tasks: %v\n", err)
	}
}

// SetToolRegistry sets the tool registry available to subagents.
func (sm *Manager) SetToolRegistry(registry *tools.ToolRegistry) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.registry = registry
}

// Spawn starts a new subagent task asynchronously.
func (sm *Manager) Spawn(ctx context.Context, task, label, originChannel, originChatID string, callback tools.AsyncCallback) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	taskID := fmt.Sprintf("agent-%d", sm.nextID)
	sm.nextID++

	// Create a new context with cancel for this specific task
	// Use the passed context as parent to ensure cleanup if caller cancels
	taskCtx, cancel := context.WithCancel(ctx) // #nosec G118

	subagentTask := &SubagentTask{
		ID:            taskID,
		Task:          task,
		Label:         label,
		OriginChannel: originChannel,
		OriginChatID:  originChatID,
		Status:        "running",
		Created:       time.Now().UnixMilli(),
		cancel:        cancel,
	}
	sm.tasks[taskID] = subagentTask
	sm.save()
	sm.mu.Unlock()
	// Start task in background
	go func(tCtx context.Context) { // #nosec G118
		defer cancel() // Ensure cancellation function is called to release resources

		// Recover from any panics to prevent crashing the entire server
		defer func() {
			if r := recover(); r != nil {
				errMsg := fmt.Sprintf("Spawn goroutine panic recovered: %v", r)
				sm.mu.Lock()
				subagentTask.Status = "failed"
				subagentTask.Result = errMsg
				subagentTask.Finished = time.Now().UnixMilli()
				sm.mu.Unlock()

				if callback != nil {
					cbCtx, cbCancel := context.WithTimeout(context.Background(), 10*time.Second) // #nosec G118
					defer cbCancel()
					callback(cbCtx, &tools.ToolResult{
						IsError: true,
						Err:     fmt.Errorf("%s", errMsg),
						ForLLM:  fmt.Sprintf("Agent panicked: %v", r),
					})
				}
			}
		}()

		result, err := sm.RunTask(tCtx, subagentTask)

		// Notify callback if present
		if callback != nil {
			var toolResult *tools.ToolResult
			if err != nil {
				toolResult = &tools.ToolResult{
					IsError: true,
					Err:     err,
					ForLLM:  fmt.Sprintf("Agent failed: %v", err),
				}
			} else {
				toolResult = &tools.ToolResult{
					ForUser: result.Content,
					ForLLM:  fmt.Sprintf("Agent completed: %s", result.Content),
				}
			}
			// Use a shorter-lived context for the callback execution.
			// Gosec ignored: using Background because this is a post-task notification
			// that should complete even if the spawn request was cancelled.
			cbCtx, cbCancel := context.WithTimeout(context.Background(), 10*time.Second) // #nosec G118
			defer cbCancel()
			callback(cbCtx, toolResult)
		}
	}(taskCtx)

	if label != "" {
		return fmt.Sprintf("Spawned agent '%s' (ID: %s) for task: %s", label, taskID, task), nil
	}
	return fmt.Sprintf("Spawned agent (ID: %s) for task: %s", taskID, task), nil
}

// RunTask executes a task synchronously.
func (sm *Manager) RunTask(ctx context.Context, task *SubagentTask) (loopResult *tools.ToolLoopResult, err error) {
	defer func() {
		sm.mu.Lock()
		task.Finished = time.Now().UnixMilli()
		if task.Status == "running" {
			task.Status = "completed"
		}
		sm.mu.Unlock()
		sm.save()
	}()

	// Recover from panics during task execution, converting them to errors
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panic recovered: %v", r)
			loopResult = &tools.ToolLoopResult{Content: err.Error()}
			sm.mu.Lock()
			task.Status = "failed"
			task.Result = err.Error()
			sm.mu.Unlock()
		}
	}()

	// Build system prompt for subagent
	systemPrompt := `You are an autonomous subagent within the RDxClaw Swarm.
Your goal is to complete the assigned task independently.
You have access to a set of tools - use them as needed.
When finished, provide a clear summary of your work.`

	messages := []providers.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: task.Task,
		},
	}

	// Run tool loop
	sm.mu.RLock()
	registry := sm.registry
	maxIter := sm.maxIterations
	sm.mu.RUnlock()

	loopResult, err = tools.RunToolLoop(ctx, tools.ToolLoopConfig{
		Provider:      sm.provider,
		Model:         sm.defaultModel,
		Tools:         registry,
		MaxIterations: maxIter,
		LLMOptions: map[string]any{
			"max_tokens":  4096,
			"temperature": 0.7,
		},
	}, messages, task.OriginChannel, task.OriginChatID)

	sm.mu.Lock()
	if err != nil {
		task.Status = "failed"
		task.Result = fmt.Sprintf("Error: %v", err)
		if ctx.Err() != nil {
			task.Status = "cancelled"
			task.Result = "Task cancelled"
		}
	} else {
		task.Status = "completed"
		task.Result = loopResult.Content
	}
	sm.mu.Unlock()
	sm.save()

	// Announce to bus
	if sm.bus != nil {
		announceContent := fmt.Sprintf("Swarm Agent '%s' (%s) finished.\nTask: %s\n\nResult:\n%s",
			task.Label, task.ID, task.Task, task.Result)
		sm.bus.PublishInbound(bus.InboundMessage{
			Channel:  "system",
			SenderID: fmt.Sprintf("swarm:%s", task.ID),
			ChatID:   fmt.Sprintf("%s:%s", task.OriginChannel, task.OriginChatID),
			Content:  announceContent,
		})
	}

	return loopResult, err
}

func (sm *Manager) GetAgent(id string) (*SubagentTask, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	task, ok := sm.tasks[id]
	return task, ok
}

func (sm *Manager) ListAgents() []*SubagentTask {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	tasks := make([]*SubagentTask, 0, len(sm.tasks))
	for _, t := range sm.tasks {
		tasks = append(tasks, t)
	}
	// Sort by creation time (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Created > tasks[j].Created
	})
	return tasks
}

func (sm *Manager) CountRunning() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	count := 0
	for _, t := range sm.tasks {
		if t.Status == "running" {
			count++
		}
	}
	return count
}

func (sm *Manager) KillAgent(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	task, ok := sm.tasks[id]
	if !ok {
		return fmt.Errorf("agent not found")
	}

	if task.Status != "running" {
		return fmt.Errorf("agent is not running (status: %s)", task.Status)
	}

	if task.cancel != nil {
		task.cancel()
	}
	task.Status = "cancelled"
	sm.save()
	return nil
}
