// RDxClaw - High-performance Agentic AI Framework for the Edge
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 rdxclaw contributors

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/channels"
	"github.com/Sterlites/RDxClaw/pkg/config"
	"github.com/Sterlites/RDxClaw/pkg/constants"
	"github.com/Sterlites/RDxClaw/pkg/knowledge"
	"github.com/Sterlites/RDxClaw/pkg/logger"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/Sterlites/RDxClaw/pkg/session"
	"github.com/Sterlites/RDxClaw/pkg/state"
	"github.com/Sterlites/RDxClaw/pkg/swarm"
	"github.com/Sterlites/RDxClaw/pkg/tools"
	"github.com/Sterlites/RDxClaw/pkg/utils"
)

type AgentLoop struct {
	bus            *bus.MessageBus
	providerMu     sync.RWMutex
	provider       providers.LLMProvider
	cfg            *config.Config
	fallbackIndex  int
	workspace      string
	model          string
	maxTokens      int
	temperature    float64
	contextWindow  int // Maximum context window size in tokens
	maxIterations  int
	sessions       *session.SessionManager
	state          *state.Manager
	contextBuilder *ContextBuilder
	tools          *tools.ToolRegistry
	running        atomic.Bool
	summarizing    sync.Map // Tracks which sessions are currently being summarized
	channelManager *channels.Manager
	swarmManager   *swarm.Manager

	// Telemetry
	telemetryMu    sync.RWMutex
	lastLatency    *LatencyStats
	sessionStats   map[string][]LatencyStats
	overallTotal   LatencyStats
	overallCount   int
}

type LatencyStats struct {
	TotalMS           int64            `json:"total_ms"`
	StartupMS         int64            `json:"startup_ms"`
	ContextBuildMS    int64            `json:"context_build_ms"`
	LLMCallsMS        int64            `json:"llm_calls_ms"`
	ToolExecMS        int64            `json:"tool_exec_ms"`
	ResponsePrepareMS int64            `json:"response_prepare_ms"`
	IterationCount    int              `json:"iteration_count"`
	Turns             []IterationStats `json:"turns,omitempty"`
	Timestamp         time.Time        `json:"timestamp"`
}

type IterationStats struct {
	Iteration          int   `json:"iteration"`
	LLMMS              int64 `json:"llm_ms"`
	ToolsMS            int64 `json:"tools_ms"`
	ProviderDurationMS int64 `json:"provider_duration_ms"`
}

type AverageStats struct {
	TotalMS           float64 `json:"total_ms"`
	StartupMS         float64 `json:"startup_ms"`
	ContextBuildMS    float64 `json:"context_build_ms"`
	LLMCallsMS        float64 `json:"llm_calls_ms"`
	ToolExecMS        float64 `json:"tool_exec_ms"`
	ResponsePrepareMS float64 `json:"response_prepare_ms"`
	AverageIterations float64 `json:"average_iterations"`
	Count             int     `json:"count"`
}

// processOptions configures how a message is processed
type processOptions struct {
	SessionKey      string // Session identifier for history/context
	Channel         string // Target channel for tool execution
	ChatID          string // Target chat ID for tool execution
	UserMessage     string // User message content (may include prefix)
	DefaultResponse string // Response when LLM returns empty
	EnableSummary   bool   // Whether to trigger summarization
	SendResponse    bool   // Whether to send response via bus
	NoHistory       bool   // If true, don't load session history (for heartbeat)
	StreamCallback  func(string) // Optional callback for intermediate chunks/thoughts
}

// createToolRegistry creates a tool registry with common tools.
// This is shared between main agent and subagents.
func createToolRegistry(workspace string, restrict bool, cfg *config.Config, msgBus *bus.MessageBus) *tools.ToolRegistry {
	registry := tools.NewToolRegistry()

	// File system tools
	registry.Register(tools.NewReadFileTool(workspace, restrict))
	registry.Register(tools.NewWriteFileTool(workspace, restrict))
	registry.Register(tools.NewListDirTool(workspace, restrict))
	registry.Register(tools.NewEditFileTool(workspace, restrict))
	registry.Register(tools.NewAppendFileTool(workspace, restrict))

	// Shell execution
	registry.Register(tools.NewExecTool(workspace, restrict))

	if searchTool := tools.NewWebSearchTool(tools.WebSearchToolOptions{
		BraveAPIKey:          cfg.Tools.Web.Brave.APIKey,
		BraveMaxResults:      cfg.Tools.Web.Brave.MaxResults,
		BraveEnabled:         cfg.Tools.Web.Brave.Enabled,
		DuckDuckGoMaxResults: cfg.Tools.Web.DuckDuckGo.MaxResults,
		DuckDuckGoEnabled:    cfg.Tools.Web.DuckDuckGo.Enabled,
	}); searchTool != nil {
		registry.Register(searchTool)
	}
	registry.Register(tools.NewWebFetchTool(10000))

	// Hardware tools (I2C, SPI) - Linux only, returns error on other platforms
	registry.Register(tools.NewI2CTool())
	registry.Register(tools.NewSPITool())

	// Message tool - available to both agent and subagent
	// Subagent uses it to communicate directly with user
	messageTool := tools.NewMessageTool()
	messageTool.SetSendCallback(func(channel, chatID, content string) error {
		msgBus.PublishOutbound(bus.OutboundMessage{
			Channel: channel,
			ChatID:  chatID,
			Content: content,
		})
		return nil
	})
	registry.Register(messageTool)

	// Knowledge Tool (RAG)
	knowledgeDir := filepath.Join(workspace, "knowledge")
	// Initialize store (ignore error for now, just log if fails)
	if store, err := knowledge.NewStore(knowledgeDir); err == nil {
		registry.Register(tools.NewKnowledgeTool(store))
	} else {
		// We can't use logger here easily as we don't pass it context, but we can print to stderr or just skip
		// Better to just skip for now or use global logger if available
		// logger.Warn("Failed to init knowledge store: " + err.Error())
	}

	return registry
}

func NewAgentLoop(cfg *config.Config, msgBus *bus.MessageBus, provider providers.LLMProvider) *AgentLoop {
	workspace := cfg.WorkspacePath()
	if err := os.MkdirAll(workspace, 0750); err != nil { // #nosec G104
		logger.ErrorCF("agent", "Failed to create workspace directory", map[string]interface{}{"error": err.Error(), "path": workspace})
	}

	restrict := cfg.Agents.Defaults.RestrictToWorkspace

	// Create tool registry for main agent
	toolsRegistry := createToolRegistry(workspace, restrict, cfg, msgBus)

	// Create subagent/swarm manager with its own tool registry
	swarmManager := swarm.NewManager(provider, cfg.Agents.Defaults.Model, workspace, msgBus)
	subagentTools := createToolRegistry(workspace, restrict, cfg, msgBus)
	// Subagent doesn't need spawn/subagent tools to avoid recursion
	swarmManager.SetToolRegistry(subagentTools)

	// Register spawn tool (for main agent)
	spawnTool := swarm.NewSpawnTool(swarmManager)
	toolsRegistry.Register(spawnTool)

	// Register subagent tool (synchronous execution)
	subagentTool := swarm.NewSubagentTool(swarmManager)
	toolsRegistry.Register(subagentTool)

	// Register swarm tool (management)
	swarmTool := swarm.NewSwarmTool(swarmManager)
	toolsRegistry.Register(swarmTool)

	sessionsManager := session.NewSessionManager(filepath.Join(workspace, "sessions"))

	// Create state manager for atomic state persistence
	stateManager := state.NewManager(workspace)

	// Create context builder and set tools registry
	contextBuilder := NewContextBuilder(workspace)
	contextBuilder.SetToolsRegistry(toolsRegistry)

	al := &AgentLoop{
		bus:            msgBus,
		provider:       provider,
		cfg:            cfg,
		fallbackIndex:  -1,
		workspace:      workspace,
		model:          cfg.Agents.Defaults.Model,
		maxTokens:      cfg.Agents.Defaults.MaxTokens,
		temperature:    cfg.Agents.Defaults.Temperature,
		contextWindow:  cfg.Agents.Defaults.MaxTokens, // For context build logic
		maxIterations:  cfg.Agents.Defaults.MaxToolIterations,
		sessions:       sessionsManager,
		state:          stateManager,
		contextBuilder: contextBuilder,
		tools:          toolsRegistry,
		summarizing:    sync.Map{},
		swarmManager:   swarmManager,
		sessionStats:   make(map[string][]LatencyStats),
	}

	// Load long-term averages from persistent state
	global := stateManager.GetGlobalTelemetry()
	al.overallTotal = LatencyStats{
		TotalMS:           global.TotalMS,
		StartupMS:         global.StartupMS,
		ContextBuildMS:    global.ContextBuildMS,
		LLMCallsMS:        global.LLMCallsMS,
		ToolExecMS:        global.ToolExecMS,
		ResponsePrepareMS: global.ResponsePrepareMS,
		IterationCount:    int(global.IterationCount),
	}
	al.overallCount = global.Count

	return al
}

func (al *AgentLoop) Run(ctx context.Context) error {
	al.running.Store(true)

	for al.running.Load() {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, ok := al.bus.ConsumeInbound(ctx)
			if !ok {
				continue
			}

			response, err := al.processMessage(ctx, msg)
			if err != nil {
				response = fmt.Sprintf("Error processing message: %v", err)
			}

			if response != "" {
				// Check if the message tool already sent a response during this round.
				// If so, skip publishing to avoid duplicate messages to the user.
				alreadySent := false
				if tool, ok := al.tools.Get("message"); ok {
					if mt, ok := tool.(*tools.MessageTool); ok {
						alreadySent = mt.HasSentInRound()
					}
				}

				if !alreadySent {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: msg.Channel,
						ChatID:  msg.ChatID,
						Content: response,
					})
				}
			}
		}
	}

	return nil
}

func (al *AgentLoop) Stop() {
	al.running.Store(false)
}

func (al *AgentLoop) IsRunning() bool {
	return al.running.Load()
}

func (al *AgentLoop) RegisterTool(tool tools.Tool) {
	al.tools.Register(tool)
}

func (al *AgentLoop) SetChannelManager(cm *channels.Manager) {
	al.channelManager = cm
}

func (al *AgentLoop) GetSwarmManager() *swarm.Manager {
	return al.swarmManager
}

func (al *AgentLoop) recordTelemetry(sessionKey string, stats LatencyStats) {
	al.telemetryMu.Lock()
	defer al.telemetryMu.Unlock()

	al.lastLatency = &stats
	al.sessionStats[sessionKey] = append(al.sessionStats[sessionKey], stats)

	al.overallTotal.TotalMS += stats.TotalMS
	al.overallTotal.StartupMS += stats.StartupMS
	al.overallTotal.ContextBuildMS += stats.ContextBuildMS
	al.overallTotal.LLMCallsMS += stats.LLMCallsMS
	al.overallTotal.ToolExecMS += stats.ToolExecMS
	al.overallTotal.ResponsePrepareMS += stats.ResponsePrepareMS
	al.overallTotal.IterationCount += stats.IterationCount
	al.overallCount++

	// Atomic persistence for cross-restart data
	go func() {
		if err := al.state.UpdateGlobalTelemetry(
			stats.TotalMS, stats.StartupMS, stats.ContextBuildMS,
			stats.LLMCallsMS, stats.ToolExecMS, stats.ResponsePrepareMS,
			stats.IterationCount,
		); err != nil {
			logger.WarnCF("agent", "Failed to persist telemetry: %v", map[string]interface{}{"error": err.Error()})
		}
	}()
}

func (al *AgentLoop) GetTelemetry(sessionKey string) (last *LatencyStats, sessAvg AverageStats, overallAvg AverageStats) {
	al.telemetryMu.RLock()
	defer al.telemetryMu.RUnlock()

	last = al.lastLatency

	// Session averages
	if stats, ok := al.sessionStats[sessionKey]; ok && len(stats) > 0 {
		var total LatencyStats
		for _, s := range stats {
			total.TotalMS += s.TotalMS
			total.StartupMS += s.StartupMS
			total.ContextBuildMS += s.ContextBuildMS
			total.LLMCallsMS += s.LLMCallsMS
			total.ToolExecMS += s.ToolExecMS
			total.ResponsePrepareMS += s.ResponsePrepareMS
			total.IterationCount += s.IterationCount
		}
		count := float64(len(stats))
		sessAvg = AverageStats{
			TotalMS:           float64(total.TotalMS) / count,
			StartupMS:         float64(total.StartupMS) / count,
			ContextBuildMS:    float64(total.ContextBuildMS) / count,
			LLMCallsMS:        float64(total.LLMCallsMS) / count,
			ToolExecMS:        float64(total.ToolExecMS) / count,
			ResponsePrepareMS: float64(total.ResponsePrepareMS) / count,
			AverageIterations: float64(total.IterationCount) / count,
			Count:             len(stats),
		}
	}

	// Overall averages
	if al.overallCount > 0 {
		count := float64(al.overallCount)
		overallAvg = AverageStats{
			TotalMS:           float64(al.overallTotal.TotalMS) / count,
			StartupMS:         float64(al.overallTotal.StartupMS) / count,
			ContextBuildMS:    float64(al.overallTotal.ContextBuildMS) / count,
			LLMCallsMS:        float64(al.overallTotal.LLMCallsMS) / count,
			ToolExecMS:        float64(al.overallTotal.ToolExecMS) / count,
			ResponsePrepareMS: float64(al.overallTotal.ResponsePrepareMS) / count,
			AverageIterations: float64(al.overallTotal.IterationCount) / count,
			Count:             al.overallCount,
		}
	}

	return
}

// RecordLastChannel records the last active channel for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChannel(channel string) error {
	return al.state.SetLastChannel(channel)
}

// RecordLastChatID records the last active chat ID for this workspace.
// This uses the atomic state save mechanism to prevent data loss on crash.
func (al *AgentLoop) RecordLastChatID(chatID string) error {
	return al.state.SetLastChatID(chatID)
}

func (al *AgentLoop) ProcessDirect(ctx context.Context, content, sessionKey string) (string, error) {
	return al.ProcessDirectWithChannel(ctx, content, sessionKey, "cli", "direct")
}

func (al *AgentLoop) ProcessDirectWithChannel(ctx context.Context, content, sessionKey, channel, chatID string) (string, error) {
	msg := bus.InboundMessage{
		Channel:    channel,
		SenderID:   "cron",
		ChatID:     chatID,
		Content:    content,
		SessionKey: sessionKey,
	}

	return al.processMessage(ctx, msg)
}

func (al *AgentLoop) ProcessStreamWithChannel(ctx context.Context, content, sessionKey, channel, chatID string, streamingCb func(string)) (string, error) {
	return al.runAgentLoop(ctx, processOptions{
		SessionKey:      sessionKey,
		Channel:         channel,
		ChatID:          chatID,
		UserMessage:     content,
		DefaultResponse: "I've completed processing your request, but the model did not provide a textual response. This can happen if the task was strictly action-oriented and no final summary was generated.",
		EnableSummary:   true,
		SendResponse:    false,
		StreamCallback:  streamingCb,
	})
}

// ProcessHeartbeat processes a heartbeat request without session history.
// Each heartbeat is independent and doesn't accumulate context.
func (al *AgentLoop) ProcessHeartbeat(ctx context.Context, content, channel, chatID string) (string, error) {
	return al.runAgentLoop(ctx, processOptions{
		SessionKey:      "heartbeat",
		Channel:         channel,
		ChatID:          chatID,
		UserMessage:     content,
		DefaultResponse: "I've completed processing your request, but the model did not provide a textual response. This can happen if the task was strictly action-oriented and no final summary was generated.",
		EnableSummary:   false,
		SendResponse:    false,
		NoHistory:       true, // Don't load session history for heartbeat
	})
}

func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	// Add message preview to log (show full content for error messages)
	var logContent string
	if strings.Contains(msg.Content, "Error:") || strings.Contains(msg.Content, "error") {
		logContent = msg.Content // Full content for errors
	} else {
		logContent = utils.Truncate(msg.Content, 80)
	}
	logger.InfoCF("agent", fmt.Sprintf("Processing message from %s:%s: %s", msg.Channel, msg.SenderID, logContent),
		map[string]interface{}{
			"channel":     msg.Channel,
			"chat_id":     msg.ChatID,
			"sender_id":   msg.SenderID,
			"session_key": msg.SessionKey,
		})

	// Route system messages to processSystemMessage
	if msg.Channel == "system" {
		return al.processSystemMessage(ctx, msg)
	}

	// Check for commands
	if response, handled := al.handleCommand(ctx, msg); handled {
		return response, nil
	}

	// Process as user message
	return al.runAgentLoop(ctx, processOptions{
		SessionKey:      msg.SessionKey,
		Channel:         msg.Channel,
		ChatID:          msg.ChatID,
		UserMessage:     msg.Content,
		DefaultResponse: "I've completed processing your request, but the model did not provide a textual response. This can happen if the task was strictly action-oriented and no final summary was generated.",
		EnableSummary:   true,
		SendResponse:    false,
	})
}

func (al *AgentLoop) processSystemMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
	// Verify this is a system message
	if msg.Channel != "system" {
		return "", fmt.Errorf("processSystemMessage called with non-system message channel: %s", msg.Channel)
	}

	logger.InfoCF("agent", "Processing system message",
		map[string]interface{}{
			"sender_id": msg.SenderID,
			"chat_id":   msg.ChatID,
		})

	// Parse origin channel from chat_id (format: "channel:chat_id")
	var originChannel string
	if idx := strings.Index(msg.ChatID, ":"); idx > 0 {
		originChannel = msg.ChatID[:idx]
	} else {
		// Fallback
		originChannel = "cli"
	}

	// Extract subagent result from message content
	// Format: "Task 'label' completed.\n\nResult:\n<actual content>"
	content := msg.Content
	if idx := strings.Index(content, "Result:\n"); idx >= 0 {
		content = content[idx+8:] // Extract just the result part
	}

	// Skip internal channels - only log, don't send to user
	if constants.IsInternalChannel(originChannel) {
		logger.InfoCF("agent", "Subagent completed (internal channel)",
			map[string]interface{}{
				"sender_id":   msg.SenderID,
				"content_len": len(content),
				"channel":     originChannel,
			})
		return "", nil
	}

	// Agent acts as dispatcher only - subagent handles user interaction via message tool
	// Don't forward result here, subagent should use message tool to communicate with user
	logger.InfoCF("agent", "Subagent completed",
		map[string]interface{}{
			"sender_id":   msg.SenderID,
			"channel":     originChannel,
			"content_len": len(content),
		})

	// Agent only logs, does not respond to user
	return "", nil
}

// runAgentLoop is the core message processing logic.
// It handles context building, LLM calls, tool execution, and response handling.
func (al *AgentLoop) runAgentLoop(ctx context.Context, opts processOptions) (string, error) {
	// 0. Record last channel for heartbeat notifications (skip internal channels)
	if opts.Channel != "" && opts.ChatID != "" {
		// Don't record internal channels (cli, system, subagent)
		if !constants.IsInternalChannel(opts.Channel) {
			channelKey := fmt.Sprintf("%s:%s", opts.Channel, opts.ChatID)
			if err := al.RecordLastChannel(channelKey); err != nil {
				logger.WarnCF("agent", "Failed to record last channel: %v", map[string]interface{}{"error": err.Error()})
			}
		}
	}

	start := time.Now()
	var stats LatencyStats
	stats.Timestamp = start

	// 0. Update tool contexts
	al.updateToolContexts(opts.Channel, opts.ChatID)
	stats.StartupMS = time.Since(start).Milliseconds()

	// 1. Build messages (skip history for heartbeat)
	contextStart := time.Now()
	var history []providers.Message
	var summary string
	if !opts.NoHistory {
		history = al.sessions.GetHistory(opts.SessionKey)
		summary = al.sessions.GetSummary(opts.SessionKey)
	}
	messages := al.contextBuilder.BuildMessages(
		history,
		summary,
		opts.UserMessage,
		nil,
		opts.Channel,
		opts.ChatID,
	)
	stats.ContextBuildMS = time.Since(contextStart).Milliseconds()

	// 2. Save user message to session
	al.sessions.AddMessage(opts.SessionKey, "user", opts.UserMessage)

	// 3. Run LLM iteration loop
	finalContent, iteration, llmMS, toolMS, turns, err := al.runLLMIteration(ctx, messages, opts)
	stats.IterationCount = iteration
	stats.LLMCallsMS = llmMS
	stats.ToolExecMS = toolMS
	stats.Turns = turns

	if err != nil {
		return "", err
	}

	// 4. Handle empty response
	respStart := time.Now()
	// If iteration > 0, we might have already sent intermediate responses
	if finalContent == "" && iteration == 0 {
		finalContent = opts.DefaultResponse
	}

	// 5. Save final assistant message to session if it hasn't been saved yet.
	history = al.sessions.GetHistory(opts.SessionKey)
	isDuplicate := false
	if len(history) > 0 {
		// Look for the last assistant message in history to check for duplication
		// Note: The history might end with a tool message, so we check the last assistant entry
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].Role == "assistant" {
				if history[i].Content == finalContent {
					isDuplicate = true
				}
				break
			}
		}
	}

	if !isDuplicate && finalContent != "" {
		al.sessions.AddMessage(opts.SessionKey, "assistant", finalContent)
	}
	_ = al.sessions.Save(opts.SessionKey) // #nosec G104

	// 6. Optional: summarization
	if opts.EnableSummary {
		al.maybeSummarize(opts.SessionKey, opts.Channel, opts.ChatID)
	}

	// 7. Optional: send response via bus
	// Only send if it's not a duplicate of what was already sent or saved
	if opts.SendResponse && !isDuplicate && finalContent != "" {
		al.bus.PublishOutbound(bus.OutboundMessage{
			Channel: opts.Channel,
			ChatID:  opts.ChatID,
			Content: finalContent,
		})
	}

	stats.ResponsePrepareMS = time.Since(respStart).Milliseconds()
	stats.TotalMS = time.Since(start).Milliseconds()

	// 8. Record Telemetry
	if !opts.NoHistory {
		al.recordTelemetry(opts.SessionKey, stats)
	}

	// 9. Log response
	responsePreview := utils.Truncate(finalContent, 120)
	logger.InfoCF("agent", fmt.Sprintf("Response: %s", responsePreview),
		map[string]interface{}{
			"session_key":  opts.SessionKey,
			"iterations":   iteration,
			"total_ms":     stats.TotalMS,
			"llm_ms":       stats.LLMCallsMS,
			"tool_ms":      stats.ToolExecMS,
			"final_length": len(finalContent),
		})

	return finalContent, nil
}

func (al *AgentLoop) rotateProvider(ctx context.Context, channel, chatID string) bool {
	al.providerMu.Lock()
	defer al.providerMu.Unlock()

	fallbacks := al.cfg.Agents.Defaults.Fallbacks
	if len(fallbacks) == 0 {
		return false
	}

	nextIndex := al.fallbackIndex + 1
	if nextIndex >= len(fallbacks) {
		return false // completely exhausted all fallbacks
	}

	fallback := fallbacks[nextIndex]

	// Create a deep copy of config using JSON
	cfgData, _ := json.Marshal(al.cfg)
	var tempCfg config.Config
	_ = json.Unmarshal(cfgData, &tempCfg)

	// Override specific fields needed for the fallback
	tempCfg.Agents.Defaults.Provider = fallback.Provider
	tempCfg.Agents.Defaults.Model = fallback.Model

	// Determine which provider to update
	switch strings.ToLower(fallback.Provider) {
	case "openai", "gpt":
		if fallback.APIKey != "" { tempCfg.Providers.OpenAI.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.OpenAI.APIBase = fallback.APIBase }
	case "anthropic", "claude":
		if fallback.APIKey != "" { tempCfg.Providers.Anthropic.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.Anthropic.APIBase = fallback.APIBase }
	case "gemini", "google":
		if fallback.APIKey != "" { tempCfg.Providers.Gemini.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.Gemini.APIBase = fallback.APIBase }
	case "groq":
		if fallback.APIKey != "" { tempCfg.Providers.Groq.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.Groq.APIBase = fallback.APIBase }
	case "openrouter":
		if fallback.APIKey != "" { tempCfg.Providers.OpenRouter.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.OpenRouter.APIBase = fallback.APIBase }
	case "vllm":
		if fallback.APIKey != "" { tempCfg.Providers.VLLM.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.VLLM.APIBase = fallback.APIBase }
	case "deepseek":
		if fallback.APIKey != "" { tempCfg.Providers.DeepSeek.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.DeepSeek.APIBase = fallback.APIBase }
	case "nvidia":
		if fallback.APIKey != "" { tempCfg.Providers.Nvidia.APIKey = fallback.APIKey }
		if fallback.APIBase != "" { tempCfg.Providers.Nvidia.APIBase = fallback.APIBase }
	}

	newProvider, err := providers.CreateProvider(&tempCfg)
	if err != nil {
		logger.ErrorCF("agent", "Failed to construct fallback provider", map[string]interface{}{"error": err.Error(), "provider": fallback.Provider})
		return false
	}

	al.provider = newProvider
	al.model = fallback.Model
	al.fallbackIndex = nextIndex

	logger.WarnCF("agent", "Rotated provider due to quota limits", map[string]interface{}{
		"new_provider": fallback.Provider,
		"new_model":    fallback.Model,
	})

	// Notify Mission Control
	al.bus.PublishOutbound(bus.OutboundMessage{
		Channel: "system",
		ChatID:  "logs",
		Content: fmt.Sprintf("⚠️ Quota exhausted. Automatically switching to fallback provider: %s (%s)", fallback.Provider, fallback.Model),
	})

	return true
}

// runLLMIteration executes the LLM call loop with tool handling.
// Returns the final content, iteration count, LLM time, tool time, turn stats, and any error.
//
// This is the core agent loop. It implements SoTA patterns 
// for production-grade run loops:
//   - Error classification with distinct recovery paths per error kind
//   - Exponential backoff with jitter for transient/rate-limit errors
//   - Multi-layer context overflow recovery (compaction → tool result truncation → give up)
//   - Hard outer-loop guard to prevent runaway retries
//   - Per-turn usage accumulation for accurate context-size reporting
//   - Tool result size guards to prevent context blowout
//   - Abort/cancellation checks between iterations and tool calls
//   - Capped preamble nudging to prevent infinite nudge loops
func (al *AgentLoop) runLLMIteration(ctx context.Context, messages []providers.Message, opts processOptions) (string, int, int64, int64, []IterationStats, error) {
	iteration := 0
	var finalContent string
	var llmTotalMS int64
	var toolTotalMS int64
	var turns []IterationStats
	var response *providers.LLMResponse
	var llmDur int64

	// SoTA: Usage tracking across all LLM calls in this turn
	usage := &UsageAccumulator{}

	// SoTA: Multi-layer overflow recovery state
	overflowCompactionAttempts := 0
	toolResultTruncationAttempted := false

	// SoTA: Preamble nudge counter (capped at MaxConsecutiveNudges)
	consecutiveNudges := 0

	// SoTA: Hard outer-loop guard to prevent runaway retries.
	// maxIterations governs "useful" iterations; this guard covers retry overhead.
	hardLoopLimit := al.maxIterations + 8
	totalLoops := 0

	for iteration < al.maxIterations {
		iteration++
		totalLoops++

		// SoTA: Hard outer-loop guard
		if totalLoops > hardLoopLimit {
			logger.ErrorCF("agent", "Hard loop limit exceeded",
				map[string]interface{}{
					"total_loops": totalLoops,
					"hard_limit":  hardLoopLimit,
					"iteration":   iteration,
				})
			if finalContent == "" {
				finalContent = "Request failed after repeated internal retries. Please try again or start a fresh session."
			}
			break
		}

		// SoTA: Check context cancellation before each iteration
		select {
		case <-ctx.Done():
			logger.WarnCF("agent", "Context cancelled between iterations",
				map[string]interface{}{"iteration": iteration})
			return finalContent, iteration, llmTotalMS, toolTotalMS, turns, ctx.Err()
		default:
		}

		logger.DebugCF("agent", "LLM iteration",
			map[string]interface{}{
				"iteration":  iteration,
				"max":        al.maxIterations,
				"total_loops": totalLoops,
			})

		// Build tool definitions
		providerToolDefs := al.tools.ToProviderDefs()

		// Log LLM request details
		logger.DebugCF("agent", "LLM request",
			map[string]interface{}{
				"iteration":         iteration,
				"model":             al.model,
				"messages_count":    len(messages),
				"tools_count":       len(providerToolDefs),
				"max_tokens":        al.maxTokens,
				"temperature":       al.temperature,
				"system_prompt_len": len(messages[0].Content),
			})

		// Log full messages (detailed)
		logger.DebugCF("agent", "Full LLM request",
			map[string]interface{}{
				"iteration":     iteration,
				"messages_json": formatMessagesForLog(messages),
				"tools_json":    formatToolsForLog(providerToolDefs),
			})

		// ── SoTA: LLM call with error-classified retry ──
		var err error
		llmStart := time.Now()
		maxRetries := 3 // Attempts beyond the first try
		for retry := 0; retry <= maxRetries; retry++ {
			// Call LLM with heartbeat for user feedback
			stopHeartbeat := al.startHeartbeat(ctx, opts.StreamCallback, "thinking")
			al.providerMu.RLock()
			currentProvider := al.provider
			currentModel := al.model
			al.providerMu.RUnlock()

			response, err = currentProvider.Chat(ctx, messages, providerToolDefs, currentModel, map[string]interface{}{
				"max_tokens":  al.maxTokens,
				"temperature": al.temperature,
			})
			stopHeartbeat()

			if err == nil {
				break // Success
			}

			errKind := classifyLLMError(err)
			logger.WarnCF("agent", "LLM call error classified",
				map[string]interface{}{
					"error":     err.Error(),
					"kind":      errKind.String(),
					"retry":     retry,
					"retryable": isRetryable(errKind),
				})

			// ── Context overflow: compaction recovery ──
			if errKind == ErrContextOverflow && retry < maxRetries {
				if overflowCompactionAttempts >= MaxOverflowCompactionAttempts {
					// SoTA: Layer 3 — exhausted compaction budget, try tool result truncation
					if !toolResultTruncationAttempted {
						toolResultTruncationAttempted = true
						truncated := al.truncateOversizedToolResults(messages)
						if truncated > 0 {
							logger.WarnCF("agent", "Truncated oversized tool results as overflow recovery",
								map[string]interface{}{"truncated_count": truncated})
							continue
						}
					}
					// Give up on context recovery
					logger.ErrorCF("agent", "Context overflow recovery exhausted",
						map[string]interface{}{
							"compaction_attempts":        overflowCompactionAttempts,
							"tool_truncation_attempted": toolResultTruncationAttempted,
						})
					break
				}

				overflowCompactionAttempts++
				logger.WarnCF("agent", "Context overflow, attempting compaction",
					map[string]interface{}{
						"error":    err.Error(),
						"attempt":  overflowCompactionAttempts,
						"max":      MaxOverflowCompactionAttempts,
					})

				// Notify user on first attempt
				if overflowCompactionAttempts == 1 && !constants.IsInternalChannel(opts.Channel) {
					if opts.StreamCallback != nil {
						opts.StreamCallback("⚠️ Context window exceeded. Compressing history and retrying...")
					} else if opts.SendResponse {
						al.bus.PublishOutbound(bus.OutboundMessage{
							Channel: opts.Channel,
							ChatID:  opts.ChatID,
							Content: "⚠️ Context window exceeded. Compressing history and retrying...",
						})
					}
				}

				// Layer 1: Force compression
				al.forceCompression(opts.SessionKey)

				// Rebuild messages from compressed history
				newHistory := al.sessions.GetHistory(opts.SessionKey)
				newSummary := al.sessions.GetSummary(opts.SessionKey)
				messages = al.contextBuilder.BuildMessages(
					newHistory,
					newSummary,
					"", // Empty — history already contains the relevant messages
					nil,
					opts.Channel,
					opts.ChatID,
				)
				continue
			}

			// ── Quota Exceeded: provider fallback rotation ──
			if errKind == ErrQuotaExceeded {
				if al.rotateProvider(ctx, opts.Channel, opts.ChatID) {
					// We successfully rotated the provider, so we can immediately retry
					logger.WarnCF("agent", "Retrying with new fallback provider", map[string]interface{}{
						"iteration": iteration,
						"retry":     retry,
					})
					continue
				}
				// If rotation failed or exhausted, we treat it as fatal and break
				logger.ErrorCF("agent", "Quota exceeded and fallback rotation failed or exhausted", map[string]interface{}{
					"error": err.Error(),
				})
				break
			}

			// ── Rate limit / Transient: exponential backoff ──
			if (errKind == ErrRateLimit || errKind == ErrTransient || errKind == ErrTimeout) && retry < maxRetries {
				backoff := computeBackoff(DefaultBackoffPolicy, retry+1)
				logger.WarnCF("agent", "Backing off before retry",
					map[string]interface{}{
						"kind":       errKind.String(),
						"retry":      retry,
						"backoff_ms": backoff.Milliseconds(),
					})
				if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
					return finalContent, iteration, llmTotalMS, toolTotalMS, turns, sleepErr
				}
				continue
			}

			// ── Fatal / Auth / Unknown: don't retry ──
			if errKind == ErrFatal || errKind == ErrAuth {
				logger.ErrorCF("agent", "Non-retryable LLM error",
					map[string]interface{}{
						"kind":  errKind.String(),
						"error": err.Error(),
					})
				break
			}

			// Unknown but retryable — simple retry without backoff
			if retry < maxRetries {
				continue
			}
			break
		}

		if err != nil {
			logger.ErrorCF("agent", "LLM call failed after retries",
				map[string]interface{}{
					"iteration": iteration,
					"error":     err.Error(),
					"kind":      classifyLLMError(err).String(),
				})
			return "", iteration, llmTotalMS, toolTotalMS, turns, fmt.Errorf("LLM call failed after retries: %w", err)
		}

		llmDur = time.Since(llmStart).Milliseconds()
		llmTotalMS += llmDur

		// SoTA: Accumulate usage from this LLM call
		if response.Usage != nil {
			usage.Merge(response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
		}

		// Update turn content status and final content tracking
		if response.Content != "" {
			finalContent = response.Content

			// SoTA AGENT BEHAVIOR: Send assistant content/thoughts immediately.
			// This provides instant feedback to the user that the agent is working.
			if opts.StreamCallback != nil {
				opts.StreamCallback(response.Content)
			} else if opts.SendResponse && !constants.IsInternalChannel(opts.Channel) {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: opts.Channel,
					ChatID:  opts.ChatID,
					Content: response.Content,
				})
				logger.DebugCF("agent", "Sent assistant content to user", map[string]interface{}{"content_len": len(response.Content)})
			}
		}

		// Check if no tool calls — model is done or needs nudging
		if len(response.ToolCalls) == 0 {
			preamble := isPreamble(response.Content)

			// SoTA: Capped preamble nudging (max MaxConsecutiveNudges attempts)
			if (response.Content == "" || preamble) && iteration < al.maxIterations && consecutiveNudges < MaxConsecutiveNudges {
				consecutiveNudges++
				logger.InfoCF("agent", "Nudging model for substantive response",
					map[string]interface{}{
						"iteration":   iteration,
						"is_preamble": preamble,
						"nudge_count": consecutiveNudges,
						"max_nudges":  MaxConsecutiveNudges,
					})

				nudge := "Task in progress. Based on the tool results above, please provide a COMPREHENSIVE report/summary of your findings to the user. Do not simply say you will continue; provide the results now."
				if preamble {
					nudge = fmt.Sprintf("You provided a preamble: %q. Now, please COMPLETE it by providing the actual results/data from the tools above. Do not call more tools unless absolutely necessary for the user's request.", utils.Truncate(response.Content, 100))
				}

				messages = append(messages, providers.Message{
					Role:    "system",
					Content: nudge,
				})
				continue
			}

			// Truly done (or nudge cap reached)
			if consecutiveNudges >= MaxConsecutiveNudges {
				logger.WarnCF("agent", "Nudge cap reached, accepting current content",
					map[string]interface{}{"nudge_count": consecutiveNudges})
			}
			break
		}

		// Reset nudge counter since the model is actively working (making tool calls)
		consecutiveNudges = 0

		// Log tool calls
		toolNames := make([]string, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.InfoCF("agent", "LLM requested tool calls",
			map[string]interface{}{
				"tools":     toolNames,
				"count":     len(response.ToolCalls),
				"iteration": iteration,
			})

		if len(toolNames) > 0 {
			msg := fmt.Sprintf("> Executing %d actions: %s...", len(toolNames), strings.Join(toolNames, ", "))
			if opts.StreamCallback != nil {
				opts.StreamCallback(msg)
			} else if opts.SendResponse && !constants.IsInternalChannel(opts.Channel) {
				al.bus.PublishOutbound(bus.OutboundMessage{
					Channel: opts.Channel,
					ChatID:  opts.ChatID,
					Content: msg,
				})
			}
		}

		// Build assistant message with tool calls
		assistantMsg := providers.Message{
			Role:    "assistant",
			Content: response.Content,
		}
		for _, tc := range response.ToolCalls {
			argumentsJSON, _ := json.Marshal(tc.Arguments)
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, providers.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &providers.FunctionCall{
					Name:      tc.Name,
					Arguments: string(argumentsJSON),
				},
			})
		}
		messages = append(messages, assistantMsg)

		// Save assistant message with tool calls to session
		al.sessions.AddFullMessage(opts.SessionKey, assistantMsg)

		// Execute tool calls
		var turnToolMS int64
		for _, tc := range response.ToolCalls {
			// SoTA: Check context cancellation between tool calls
			select {
			case <-ctx.Done():
				logger.WarnCF("agent", "Context cancelled between tool calls",
					map[string]interface{}{"tool": tc.Name, "iteration": iteration})
				return finalContent, iteration, llmTotalMS, toolTotalMS, turns, ctx.Err()
			default:
			}

			// Log tool call with arguments preview
			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.InfoCF("agent", fmt.Sprintf("Tool call: %s(%s)", tc.Name, argsPreview),
				map[string]interface{}{
					"tool":      tc.Name,
					"iteration": iteration,
				})

			// Create async callback for tools that implement AsyncTool
			asyncCallback := func(callbackCtx context.Context, result *tools.ToolResult) {
				if !result.Silent && result.ForUser != "" {
					if opts.StreamCallback != nil {
						opts.StreamCallback(">> " + strings.ReplaceAll(result.ForUser, "\n", "\n>> "))
					} else if opts.SendResponse {
						al.bus.PublishOutbound(bus.OutboundMessage{
							Channel: opts.Channel,
							ChatID:  opts.ChatID,
							Content: result.ForUser,
						})
					}
					logger.InfoCF("agent", "Async tool completed, handling notification",
						map[string]interface{}{
							"tool":        tc.Name,
							"content_len": len(result.ForUser),
						})
				}
			}

			toolStart := time.Now()
			stopToolHeartbeat := al.startHeartbeat(ctx, opts.StreamCallback, fmt.Sprintf("executing %s", tc.Name))
			toolResult := al.tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, opts.Channel, opts.ChatID, asyncCallback)
			stopToolHeartbeat()
			toolDur := time.Since(toolStart).Milliseconds()
			turnToolMS += toolDur
			toolTotalMS += toolDur

			// Send ForUser content to user immediately if not Silent
			if !toolResult.Silent && toolResult.ForUser != "" {
				if opts.StreamCallback != nil {
					// We prefix it so the user knows it's an action/tool update
					opts.StreamCallback(">> (" + tc.Name + " output)\n>> " + strings.ReplaceAll(toolResult.ForUser, "\n", "\n>> "))
				} else if opts.SendResponse {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: opts.Channel,
						ChatID:  opts.ChatID,
						Content: toolResult.ForUser,
					})
				}
				logger.DebugCF("agent", "Sent tool result to user",
					map[string]interface{}{
						"tool":        tc.Name,
						"content_len": len(toolResult.ForUser),
					})
			}

			// Determine content for LLM based on tool result
			contentForLLM := toolResult.ForLLM
			if contentForLLM == "" && toolResult.Err != nil {
				contentForLLM = toolResult.Err.Error()
			}

			// SoTA: Tool result size guard — truncate oversized results
			if truncated, wasTruncated := truncateToolResult(contentForLLM); wasTruncated {
				logger.WarnCF("agent", "Tool result truncated (exceeds size limit)",
					map[string]interface{}{
						"tool":          tc.Name,
						"original_size": len(contentForLLM),
						"max_size":      MaxToolResultBytes,
					})
				contentForLLM = truncated
			}

			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    contentForLLM,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)

			// Save tool result message to session
			al.sessions.AddFullMessage(opts.SessionKey, toolResultMsg)
		}

		turns = append(turns, IterationStats{
			Iteration:          iteration,
			LLMMS:              llmDur,
			ToolsMS:            turnToolMS,
			ProviderDurationMS: int64(response.DurationMS),
		})
	}

	// SoTA: Log completion with usage summary
	var responseContentLen int
	if response != nil {
		responseContentLen = len(response.Content)
	}
	logger.InfoCF("agent", "LLM loop complete",
		map[string]interface{}{
			"iteration":      iteration,
			"total_loops":     totalLoops,
			"content_chars":   responseContentLen,
			"llm_calls":       usage.CallCount,
			"total_input_tok": usage.InputTokens,
			"total_output_tok": usage.OutputTokens,
		})
	if response != nil {
		turns = append(turns, IterationStats{
			Iteration:          iteration,
			LLMMS:              llmDur,
			ProviderDurationMS: int64(response.DurationMS),
		})
	}

	return finalContent, iteration, llmTotalMS, toolTotalMS, turns, nil
}

// truncateOversizedToolResults scans the message list for tool results that
// exceed the context window's fair share and truncates them in-place.
// Returns the number of messages truncated.
func (al *AgentLoop) truncateOversizedToolResults(messages []providers.Message) int {
	// Consider any tool result > 50% of context window as oversized
	maxTokens := al.contextWindow / 2
	maxChars := maxTokens * 3 // Rough chars-to-tokens estimate
	truncated := 0

	for i := range messages {
		if messages[i].Role != "tool" {
			continue
		}
		if len(messages[i].Content) > maxChars {
			origLen := len(messages[i].Content)
			messages[i].Content = messages[i].Content[:maxChars] +
				"\n...[TRUNCATED: tool result exceeded context budget]..."
			logger.WarnCF("agent", "Truncated oversized tool result in context",
				map[string]interface{}{
					"tool_call_id": messages[i].ToolCallID,
					"original_len": origLen,
					"truncated_to": maxChars,
				})
			truncated++
		}
	}
	return truncated
}

func isPreamble(content string) bool {
	if content == "" {
		return false
	}
	trimmed := strings.TrimSpace(strings.ToLower(content))
	
	// Suffix-based triggers
	if strings.HasSuffix(trimmed, ":") || 
	   strings.HasSuffix(trimmed, "following") || 
	   strings.HasSuffix(trimmed, "follows") ||
	   strings.HasSuffix(trimmed, "below") ||
	   strings.HasSuffix(trimmed, "tasks") {
		return true
	}

	// Phrase-based triggers (common in stalling models)
	stallPhrases := []string{
		"let me continue",
		"i will now",
		"i will then",
		"next i will",
		"the following tools",
		"starting the task",
		"comprehensive task",
	}
	
	for _, phrase := range stallPhrases {
		if strings.Contains(trimmed, phrase) {
			// Check if it's very short (likely just a preamble)
			if len(trimmed) < 200 {
				return true
			}
		}
	}

	return false
}

// updateToolContexts updates the context for tools that need channel/chatID info.
func (al *AgentLoop) updateToolContexts(channel, chatID string) {
	// Use ContextualTool interface instead of type assertions
	if tool, ok := al.tools.Get("message"); ok {
		if mt, ok := tool.(tools.ContextualTool); ok {
			mt.SetContext(channel, chatID)
		}
	}
	if tool, ok := al.tools.Get("spawn"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID)
		}
	}
	if tool, ok := al.tools.Get("subagent"); ok {
		if st, ok := tool.(tools.ContextualTool); ok {
			st.SetContext(channel, chatID)
		}
	}
}

// maybeSummarize triggers summarization if the session history exceeds thresholds.
func (al *AgentLoop) maybeSummarize(sessionKey, channel, chatID string) {
	newHistory := al.sessions.GetHistory(sessionKey)
	tokenEstimate := al.estimateTokens(newHistory)
	threshold := al.contextWindow * 75 / 100

	if len(newHistory) > 20 || tokenEstimate > threshold {
		if _, loading := al.summarizing.LoadOrStore(sessionKey, true); !loading {
			go func() {
				defer al.summarizing.Delete(sessionKey)
				// Notify user about optimization if not an internal channel
				if !constants.IsInternalChannel(channel) {
					al.bus.PublishOutbound(bus.OutboundMessage{
						Channel: channel,
						ChatID:  chatID,
						Content: "⚠️ Memory threshold reached. Optimizing conversation history...",
					})
				}
				al.summarizeSession(sessionKey)
			}()
		}
	}
}

// forceCompression aggressively reduces context when the limit is hit.
// It drops the oldest 50% of messages (keeping system prompt and last user message).
func (al *AgentLoop) forceCompression(sessionKey string) {
	history := al.sessions.GetHistory(sessionKey)
	if len(history) <= 4 {
		return
	}

	// Keep system prompt (usually [0]) and the very last message (user's trigger)
	// We want to drop the oldest half of the *conversation*
	// Assuming [0] is system, [1:] is conversation
	conversation := history[1 : len(history)-1]
	if len(conversation) == 0 {
		return
	}

	// Helper to find the mid-point of the conversation
	mid := len(conversation) / 2

	// New history structure:
	// 1. System Prompt
	// 2. [Summary of dropped part] - synthesized
	// 3. Second half of conversation
	// 4. Last message

	// Simplified approach for emergency: Drop first half of conversation
	// and rely on existing summary if present, or create a placeholder.

	droppedCount := mid
	keptConversation := conversation[mid:]

	newHistory := make([]providers.Message, 0)
	newHistory = append(newHistory, history[0]) // System prompt

	// Add a note about compression
	compressionNote := fmt.Sprintf("[System: Emergency compression dropped %d oldest messages due to context limit]", droppedCount)
	// If there was an existing summary, we might lose it if it was in the dropped part (which is just messages).
	// The summary is stored separately in session.Summary, so it persists!
	// We just need to ensure the user knows there's a gap.

	// We only modify the messages list here
	newHistory = append(newHistory, providers.Message{
		Role:    "system",
		Content: compressionNote,
	})

	newHistory = append(newHistory, keptConversation...)
	newHistory = append(newHistory, history[len(history)-1]) // Last message

	// Update session
	al.sessions.SetHistory(sessionKey, newHistory)
	_ = al.sessions.Save(sessionKey) // #nosec G104

	logger.WarnCF("agent", "Forced compression executed", map[string]interface{}{
		"session_key":  sessionKey,
		"dropped_msgs": droppedCount,
		"new_count":    len(newHistory),
	})
}

// GetStartupInfo returns information about loaded tools and skills for logging.
func (al *AgentLoop) GetStartupInfo() map[string]interface{} {
	info := make(map[string]interface{})
	info["model"] = al.model

	// Tools info
	tools := al.tools.List()
	info["tools"] = map[string]interface{}{
		"count": len(tools),
		"names": tools,
	}

	// Skills info
	info["skills"] = al.contextBuilder.GetSkillsInfo()

	return info
}

// formatMessagesForLog formats messages for logging
func formatMessagesForLog(messages []providers.Message) string {
	if len(messages) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, msg := range messages {
		result += fmt.Sprintf("  [%d] Role: %s\n", i, msg.Role)
		if len(msg.ToolCalls) > 0 {
			result += "  ToolCalls:\n"
			for _, tc := range msg.ToolCalls {
				result += fmt.Sprintf("    - ID: %s, Type: %s, Name: %s\n", tc.ID, tc.Type, tc.Name)
				if tc.Function != nil {
					result += fmt.Sprintf("      Arguments: %s\n", utils.Truncate(tc.Function.Arguments, 200))
				}
			}
		}
		if msg.Content != "" {
			content := utils.Truncate(msg.Content, 200)
			result += fmt.Sprintf("  Content: %s\n", content)
		}
		if msg.ToolCallID != "" {
			result += fmt.Sprintf("  ToolCallID: %s\n", msg.ToolCallID)
		}
		result += "\n"
	}
	result += "]"
	return result
}

// formatToolsForLog formats tool definitions for logging
func formatToolsForLog(tools []providers.ToolDefinition) string {
	if len(tools) == 0 {
		return "[]"
	}

	var result string
	result += "[\n"
	for i, tool := range tools {
		result += fmt.Sprintf("  [%d] Type: %s, Name: %s\n", i, tool.Type, tool.Function.Name)
		result += fmt.Sprintf("      Description: %s\n", tool.Function.Description)
		if len(tool.Function.Parameters) > 0 {
			result += fmt.Sprintf("      Parameters: %s\n", utils.Truncate(fmt.Sprintf("%v", tool.Function.Parameters), 200))
		}
	}
	result += "]"
	return result
}

// summarizeSession summarizes the conversation history for a session.
func (al *AgentLoop) summarizeSession(sessionKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	history := al.sessions.GetHistory(sessionKey)
	summary := al.sessions.GetSummary(sessionKey)

	// Keep last 4 messages for continuity
	if len(history) <= 4 {
		return
	}

	toSummarize := history[:len(history)-4]

	// Oversized Message Guard
	// Skip messages larger than 50% of context window to prevent summarizer overflow
	maxMessageTokens := al.contextWindow / 2
	validMessages := make([]providers.Message, 0)
	omitted := false

	for _, m := range toSummarize {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// Estimate tokens for this message
		msgTokens := len(m.Content) / 2 // Use safer estimate here too (2.5 -> 2 for integer division safety)
		if msgTokens > maxMessageTokens {
			omitted = true
			continue
		}
		validMessages = append(validMessages, m)
	}

	if len(validMessages) == 0 {
		return
	}

	// Multi-Part Summarization
	// Split into two parts if history is significant
	var finalSummary string
	if len(validMessages) > 10 {
		mid := len(validMessages) / 2
		part1 := validMessages[:mid]
		part2 := validMessages[mid:]

		s1, _ := al.summarizeBatch(ctx, part1, "")
		s2, _ := al.summarizeBatch(ctx, part2, "")

		// Merge them
		mergePrompt := fmt.Sprintf("Merge these two conversation summaries into one cohesive summary:\n\n1: %s\n\n2: %s", s1, s2)
		resp, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: mergePrompt}}, nil, al.model, map[string]interface{}{
			"max_tokens":  1024,
			"temperature": 0.3,
		})
		if err == nil {
			finalSummary = resp.Content
		} else {
			finalSummary = s1 + " " + s2
		}
	} else {
		finalSummary, _ = al.summarizeBatch(ctx, validMessages, summary)
	}

	if omitted && finalSummary != "" {
		finalSummary += "\n[Note: Some oversized messages were omitted from this summary for efficiency.]"
	}

	if finalSummary != "" {
		al.sessions.SetSummary(sessionKey, finalSummary)
		al.sessions.TruncateHistory(sessionKey, 4)
		_ = al.sessions.Save(sessionKey) // #nosec G104
	}
}

// summarizeBatch summarizes a batch of messages.
func (al *AgentLoop) summarizeBatch(ctx context.Context, batch []providers.Message, existingSummary string) (string, error) {
	prompt := "Provide a concise summary of this conversation segment, preserving core context and key points.\n"
	if existingSummary != "" {
		prompt += "Existing context: " + existingSummary + "\n"
	}
	prompt += "\nCONVERSATION:\n"
	for _, m := range batch {
		prompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	response, err := al.provider.Chat(ctx, []providers.Message{{Role: "user", Content: prompt}}, nil, al.model, map[string]interface{}{
		"max_tokens":  1024,
		"temperature": 0.3,
	})
	if err != nil {
		return "", err
	}
	return response.Content, nil
}

// estimateTokens estimates the number of tokens in a message list.
// Uses a safe heuristic of 2.5 characters per token to account for CJK and other
// overheads better than the previous 3 chars/token.
func (al *AgentLoop) estimateTokens(messages []providers.Message) int {
	totalChars := 0
	for _, m := range messages {
		totalChars += utf8.RuneCountInString(m.Content)
	}
	// 2.5 chars per token = totalChars * 2 / 5
	return totalChars * 2 / 5
}

func (al *AgentLoop) handleCommand(ctx context.Context, msg bus.InboundMessage) (string, bool) {
	content := strings.TrimSpace(msg.Content)
	if !strings.HasPrefix(content, "/") {
		return "", false
	}

	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", false
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/show":
		if len(args) < 1 {
			return "Usage: /show [model|channel]", true
		}
		switch args[0] {
		case "model":
			return fmt.Sprintf("Current model: %s", al.model), true
		case "channel":
			return fmt.Sprintf("Current channel: %s", msg.Channel), true
		default:
			return fmt.Sprintf("Unknown show target: %s", args[0]), true
		}

	case "/list":
		if len(args) < 1 {
			return "Usage: /list [models|channels]", true
		}
		switch args[0] {
		case "models":
			// TODO: Fetch available models dynamically if possible
			return "Available models: glm-4.7, claude-3-5-sonnet, gpt-4o (configured in config.json/env)", true
		case "channels":
			if al.channelManager == nil {
				return "Channel manager not initialized", true
			}
			channels := al.channelManager.GetEnabledChannels()
			if len(channels) == 0 {
				return "No channels enabled", true
			}
			return fmt.Sprintf("Enabled channels: %s", strings.Join(channels, ", ")), true
		default:
			return fmt.Sprintf("Unknown list target: %s", args[0]), true
		}

	case "/switch":
		if len(args) < 3 || args[1] != "to" {
			return "Usage: /switch [model|channel] to <name>", true
		}
		target := args[0]
		value := args[2]

		switch target {
		case "model":
			oldModel := al.model
			al.model = value
			return fmt.Sprintf("Switched model from %s to %s", oldModel, value), true
		case "channel":
			// This changes the 'default' channel for some operations, or effectively redirects output?
			// For now, let's just validate if the channel exists
			if al.channelManager == nil {
				return "Channel manager not initialized", true
			}
			if _, exists := al.channelManager.GetChannel(value); !exists && value != "cli" {
				return fmt.Sprintf("Channel '%s' not found or not enabled", value), true
			}

			// If message came from CLI, maybe we want to redirect CLI output to this channel?
			// That would require state persistence about "redirected channel"
			// For now, just acknowledged.
			return fmt.Sprintf("Switched target channel to %s (Note: this currently only validates existence)", value), true
		default:
			return fmt.Sprintf("Unknown switch target: %s", target), true
		}
	}

	return "", false
}

// startHeartbeat starts a background goroutine that sends periodic progress updates
// via the stream callback if the operation takes too long.
// It returns a function that should be called when the operation completes.
func (al *AgentLoop) startHeartbeat(ctx context.Context, callback func(string), label string) func() {
	if callback == nil {
		return func() {}
	}

	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		startTime := time.Now()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := int(time.Since(startTime).Seconds())
				// Use a subtle prefix for heartbeats
				callback(fmt.Sprintf(".. %s (still working, %ds) ..", label, elapsed))
			}
		}
	}()

	return func() {
		// Close done channel to stop the goroutine.
		// Use a temporary channel to avoid double-close panic if called multiple times.
		select {
		case <-done:
			// already closed
		default:
			close(done)
		}
	}
}
