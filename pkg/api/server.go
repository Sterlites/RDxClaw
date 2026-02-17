package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/agent"
	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/skills"
)

// Server is the headless REST API server for RDxClaw.
// It provides OpenAI-compatible endpoints, skill execution,
// webhook handling, and agent status.
type Server struct {
	agentLoop *agent.AgentLoop
	msgBus    *bus.MessageBus
	loader    *skills.SkillsLoader
	config    ServerConfig
	startedAt time.Time
	version   string
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	Host        string
	Port        int
	APIKey      string
	RateLimit   int // requests per minute (0 = unlimited)
	CORSOrigins []string
}

// NewServer creates a new API server instance.
func NewServer(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, loader *skills.SkillsLoader, cfg ServerConfig) *Server {
	return &Server{
		agentLoop: agentLoop,
		msgBus:    msgBus,
		loader:    loader,
		config:    cfg,
		startedAt: time.Now(),
		version:   "1.0.0",
	}
}

// Start starts the API server (blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletion)
	mux.HandleFunc("POST /v1/skills/{skill}/execute", s.handleSkillExecute)
	mux.HandleFunc("POST /v1/webhooks/", s.handleWebhook) // catch-all for webhook paths
	mux.HandleFunc("GET /v1/status", s.handleStatus)
	mux.HandleFunc("GET /v1/skills", s.handleListSkills)
	mux.HandleFunc("GET /v1/skills", s.handleListSkills)
	mux.HandleFunc("GET /v1/agents", s.handleListAgents)
	mux.HandleFunc("DELETE /v1/agents/{id}", s.handleKillAgent)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleHealth)

	// Apply middleware stack
	var handler http.Handler = mux
	handler = LoggingMiddleware(handler)

	if s.config.RateLimit > 0 {
		limiter := NewRateLimiter(s.config.RateLimit, time.Minute)
		handler = RateLimitMiddleware(limiter, handler)
	}

	if len(s.config.CORSOrigins) > 0 {
		handler = CORSMiddleware(s.config.CORSOrigins, handler)
	}

	handler = AuthMiddleware(s.config.APIKey, handler)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	slog.Info("API server starting", "addr", addr)
	return http.ListenAndServe(addr, handler)
}

// --- Handlers ---

func (s *Server) handleChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req ChatCompletionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "messages array is required and must not be empty")
		return
	}

	// Extract the last user message
	var userContent string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			userContent = req.Messages[i].Content
			break
		}
	}

	if userContent == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "at least one user message is required")
		return
	}

	// Generate session key
	sessionKey := req.SessionKey
	if sessionKey == "" {
		sessionKey = fmt.Sprintf("api-%d", time.Now().UnixNano())
	}

	channel := req.Channel
	if channel == "" {
		channel = "api"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, userContent, sessionKey, channel, "api")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "processing_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: response},
				FinishReason: "stop",
			},
		},
	})
}

func (s *Server) handleSkillExecute(w http.ResponseWriter, r *http.Request) {
	skillName := r.PathValue("skill")
	if skillName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "skill name is required")
		return
	}

	var req SkillExecuteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// Load the skill to verify it exists
	_, found := s.loader.LoadSkill(skillName)
	if !found {
		writeError(w, http.StatusNotFound, "skill_not_found", fmt.Sprintf("skill '%s' not found", skillName))
		return
	}

	// Build the prompt with skill context
	prompt := fmt.Sprintf("[Using skill: %s]\n\n%s", skillName, req.Input)

	sessionKey := req.SessionKey
	if sessionKey == "" {
		sessionKey = fmt.Sprintf("skill-%s-%d", skillName, time.Now().UnixNano())
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, prompt, sessionKey, "api", "api")
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusOK, SkillExecuteResponse{
			SkillName: skillName,
			Duration:  duration,
			Error:     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, SkillExecuteResponse{
		SkillName: skillName,
		Result:    response,
		Duration:  duration,
	})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Extract the webhook path (everything after /v1/webhooks/)
	webhookPath := strings.TrimPrefix(r.URL.Path, "/v1/webhooks")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "failed to read request body")
		return
	}
	defer r.Body.Close()

	// Parse body as JSON
	var bodyMap map[string]interface{}
	_ = json.Unmarshal(body, &bodyMap)

	// Extract headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	event := WebhookEvent{
		Path:      webhookPath,
		Headers:   headers,
		Body:      bodyMap,
		RawBody:   string(body),
		Timestamp: time.Now().UnixMilli(),
	}

	// Publish to message bus as an inbound message so the agent processes it
	eventJSON, _ := json.Marshal(event)
	s.msgBus.PublishInbound(bus.InboundMessage{
		Channel:    "webhook",
		SenderID:   "webhook",
		ChatID:     webhookPath,
		Content:    fmt.Sprintf("[Webhook received on %s]\n\n%s", webhookPath, string(eventJSON)),
		SessionKey: fmt.Sprintf("webhook-%s", webhookPath),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"received": true,
		"path":     webhookPath,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	startupInfo := s.agentLoop.GetStartupInfo()
	toolsInfo := startupInfo["tools"].(map[string]interface{})
	skillsInfo := startupInfo["skills"].(map[string]interface{})

	allSkills := s.loader.ListSkills()
	skillNames := make([]string, len(allSkills))
	for i, skill := range allSkills {
		skillNames[i] = skill.Name
	}

	writeJSON(w, http.StatusOK, StatusResponse{
		Status:    "ok",
		Version:   s.version,
		Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
		StartedAt: s.startedAt,
		Agent: AgentStatus{
			ToolsLoaded: toolsInfo["count"].(int),
		},
		Skills: SkillsStatus{
			Total:     skillsInfo["total"].(int),
			Available: skillsInfo["available"].(int),
			Names:     skillNames,
		},
	})
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	allSkills := s.loader.ListSkills()
	items := make([]SkillListItem, len(allSkills))
	for i, skill := range allSkills {
		items[i] = SkillListItem{
			Name:         skill.Name,
			Description:  skill.Description,
			Source:       skill.Source,
			Capabilities: skill.Capabilities,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skills": items,
		"total":  len(items),
	})
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"skills": items,
		"total":  len(items),
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	manager := s.agentLoop.GetSwarmManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, "swarm_unavailable", "swarm manager not initialized")
		return
	}

	agents := manager.ListAgents()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

func (s *Server) handleKillAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "agent id is required")
		return
	}

	manager := s.agentLoop.GetSwarmManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, "swarm_unavailable", "swarm manager not initialized")
		return
	}

	err := manager.KillAgent(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "agent_not_found", err.Error())
		} else {
			writeError(w, http.StatusBadRequest, "kill_failed", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Agent %s killed", id),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// --- Helpers ---

func decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Message: message,
			Type:    "api_error",
			Code:    code,
		},
	})
}
