package api

import (
	"time"
	"github.com/Sterlites/RDxClaw/pkg/agent"
)

// --- OpenAI-Compatible Chat Completion Types ---

// ChatCompletionRequest mirrors the OpenAI chat completion request format.
type ChatCompletionRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	SessionKey  string        `json:"session_key,omitempty"` // RDxClaw extension
	Channel     string        `json:"channel,omitempty"`     // RDxClaw extension
}

// ChatMessage represents a single message in the chat.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse mirrors the OpenAI chat completion response format.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *ChatCompletionUsage   `json:"usage,omitempty"`
}

// ChatCompletionChoice represents a single completion choice.
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage tracks token usage.
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Skill Execution Types ---

// SkillExecuteRequest triggers a skill by name with optional input.
type SkillExecuteRequest struct {
	Input      string                 `json:"input"`
	Params     map[string]interface{} `json:"params,omitempty"`
	SessionKey string                 `json:"session_key,omitempty"`
}

// SkillExecuteResponse contains the result of a skill execution.
type SkillExecuteResponse struct {
	SkillName string `json:"skill_name"`
	Result    string `json:"result"`
	Duration  int64  `json:"duration_ms"`
	Error     string `json:"error,omitempty"`
}

// --- Webhook Types ---

// WebhookEvent represents an incoming webhook payload.
type WebhookEvent struct {
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers,omitempty"`
	Body      map[string]interface{} `json:"body,omitempty"`
	RawBody   string                 `json:"raw_body,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// --- Status Types ---

// StatusResponse contains the server health and agent status.
type StatusResponse struct {
	Status       string          `json:"status"`
	Version      string          `json:"version"`
	Uptime       string          `json:"uptime"`
	StartedAt    time.Time       `json:"started_at"`
	Agent        AgentStatus     `json:"agent"`
	Skills       SkillsStatus    `json:"skills"`
	ActiveAgents int             `json:"active_agents"`
	RecentEvents []ActivityEvent `json:"recent_events,omitempty"`
	Cron         map[string]interface{} `json:"cron,omitempty"`
	System       SystemStats     `json:"system"`
	Workspace    WorkspaceStats  `json:"workspace"`
	Telemetry    *TelemetryInfo  `json:"telemetry,omitempty"`
}

type TelemetryInfo struct {
	LastResponse    *agent.LatencyStats `json:"last_response,omitempty"`
	SessionAverages agent.AverageStats  `json:"session_averages"`
	OverallAverages agent.AverageStats  `json:"overall_averages"`
}

// SystemStats contains Go runtime statistics.
type SystemStats struct {
	MemoryUsage string  `json:"memory_usage"`
	CPULoad     float64 `json:"cpu_load"` // Added for more data points
	Goroutines  int     `json:"goroutines"`
	Threads     int     `json:"threads"`      // OS Threads
	HeapObjects uint64  `json:"heap_objects"` // Total heap objects
}

// WorkspaceStats contains metrics about the RDxClaw workspace.
type WorkspaceStats struct {
	TotalFiles int    `json:"total_files"`
	Size       string `json:"size"`
}

// ActivityEvent represents a significant system event for Mission Control.
type ActivityEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`   // "info", "warning", "error", "success"
	Source    string    `json:"source"` // "agent", "api", "skill", "system"
	Message   string    `json:"message"`
}

// AgentStatus contains agent health information.
type AgentStatus struct {
	Model       string `json:"model"`
	ToolsLoaded int    `json:"tools_loaded"`
}

// SkillsStatus contains skills summary.
type SkillsStatus struct {
	Total     int      `json:"total"`
	Available int      `json:"available"`
	Names     []string `json:"names"`
}

// SkillListItem is used in the GET /v1/skills response.
type SkillListItem struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Source       string `json:"source"`
	Capabilities string `json:"capabilities,omitempty"`
}

// --- File Explorer Types ---

// FileListItem represents a file in the workspace.
type FileListItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	RelPath string    `json:"rel_path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// UpdateFileRequest is the payload for saving file content
type UpdateFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FileContentResponse contains the content of a requested file.
type FileContentResponse struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ErrorResponse is the standard API error format.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}


// ErrorDetail contains error information.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}
