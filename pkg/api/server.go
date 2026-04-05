package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sterlites/RDxClaw/pkg/agent"
	"github.com/Sterlites/RDxClaw/pkg/bus"
	"github.com/Sterlites/RDxClaw/pkg/skills"
)

//go:embed web/*
var embeddedWebFS embed.FS

type Server struct {
	agentLoop *agent.AgentLoop
	msgBus    *bus.MessageBus
	loader    *skills.SkillsLoader
	config    ServerConfig
	startedAt time.Time
	version   string
	events    []ActivityEvent
	eventsMu  sync.RWMutex
	workspace string
	clients   map[chan string]bool
	clientsMu sync.Mutex
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
func NewServer(agentLoop *agent.AgentLoop, msgBus *bus.MessageBus, loader *skills.SkillsLoader, workspace string, cfg ServerConfig) *Server {
	s := &Server{
		agentLoop: agentLoop,
		msgBus:    msgBus,
		loader:    loader,
		workspace: workspace,
		config:    cfg,
		startedAt: time.Now(),
		version:   "1.0.0",
		events:    make([]ActivityEvent, 0),
		clients:   make(map[chan string]bool),
	}
	s.recordEvent("system", "success", "RDxClaw Mission Control initialized")
	return s
}

func (s *Server) recordEvent(source, eventType, message string) {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	event := ActivityEvent{
		Timestamp: time.Now(),
		Source:    source,
		Type:      eventType,
		Message:   message,
	}

	// Keep only the last 50 events
	s.events = append(s.events, event)
	if len(s.events) > 50 {
		s.events = s.events[1:]
	}

	// SSE Broadcast
	s.broadcast("activity", event)
}

func (s *Server) broadcast(eventType string, data interface{}) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	payload, err := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	if err != nil {
		slog.Error("failed to marshal broadcast payload", "error", err)
		return
	}

	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(payload))
	for client := range s.clients {
		select {
		case client <- msg:
		default:
			// Client channel full, skip
		}
	}
}

// Start starts the API server (blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Embed Frontend Web UI
	webContent, err := fs.Sub(embeddedWebFS, "web")
	if err != nil {
		return fmt.Errorf("failed to prepare embedded web fs: %v", err)
	}
	
	// Serve static files from the embedded FS. 
	// The pattern "GET /" acts as a catch-all for GET requests not matched by other routes.
	
	// Serve static files from the embedded FS.
	fileServer := http.FileServer(http.FS(webContent))
	
	// Specific handler for root to ensure index.html is served correctly without redirect loops
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		index, err := fs.ReadFile(webContent, "index.html")
		if err != nil {
			slog.Error("failed to read index.html", "error", err)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write(index); err != nil {
			slog.Error("failed to write response", "error", err)
		}
	})

	// Serve other static files (css, js, etc.)
	mux.Handle("GET /", fileServer)

	// Register routes
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletion)
	mux.HandleFunc("POST /v1/skills/{skill}/execute", s.handleSkillExecute)
	mux.HandleFunc("POST /v1/webhooks/", s.handleWebhook) // catch-all for webhook paths
	mux.HandleFunc("GET /v1/status", s.handleStatus)
	mux.HandleFunc("GET /v1/skills", s.handleListSkills)
	mux.HandleFunc("GET /v1/agents", s.handleListAgents)
	mux.HandleFunc("DELETE /v1/agents/{id}", s.handleKillAgent)
	mux.HandleFunc("GET /v1/files", s.handleListFiles)
	mux.HandleFunc("GET /v1/files/content", s.handleGetFileContent)
	mux.HandleFunc("POST /v1/files/save", s.handleUpdateFileContent)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("POST /v1/sessions/resume", s.handleResumeSession)
	mux.HandleFunc("GET /v1/config/recovery", s.handleGetRecoveryConfig)
	mux.HandleFunc("POST /v1/config/recovery", s.handleUpdateRecoveryConfig)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleHealth)
	mux.HandleFunc("GET /v1/events", s.handleEvents)

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

	// Start status broadcaster
	go s.statusBroadcaster()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       24 * time.Hour,
		WriteTimeout:      24 * time.Hour,
		IdleTimeout:       120 * time.Second,
	}

	return srv.ListenAndServe()
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

	ctx, cancel := context.WithTimeout(r.Context(), 24*time.Hour)
	defer cancel()

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming unsupported")
			return
		}

		sendChunk := func(content string) {
			chunk := ChatCompletionResponse{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []ChatCompletionChoice{
					{
						Index:   0,
						Message: ChatMessage{Role: "assistant", Content: content},
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}

		_, err := s.agentLoop.ProcessStreamWithChannel(ctx, userContent, sessionKey, channel, "api", sendChunk)
		if err != nil {
			s.recordEvent("agent", "error", fmt.Sprintf("Chat error: %v", err))
			chunk := ChatCompletionResponse{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []ChatCompletionChoice{
					{
						Index:        0,
						Message:      ChatMessage{Role: "assistant", Content: fmt.Sprintf("Error: %v", err)},
						FinishReason: "error",
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
		} else {
			// Final chunk for streaming: usually empty content with stop reason
			chunk := ChatCompletionResponse{
				ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []ChatCompletionChoice{
					{
						Index:        0,
						Message:      ChatMessage{Role: "assistant", Content: ""},
						FinishReason: "stop",
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}

		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, userContent, sessionKey, channel, "api")
	if err != nil {
		s.recordEvent("agent", "error", fmt.Sprintf("Chat error: %v", err))
		writeError(w, http.StatusInternalServerError, "processing_error", err.Error())
		return
	}

	s.recordEvent("agent", "info", "Processed user request")

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
	ctx, cancel := context.WithTimeout(r.Context(), 24*time.Hour)
	defer cancel()

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, prompt, sessionKey, "api", "api")
	duration := time.Since(startTime).Milliseconds()

	if err != nil {
		s.recordEvent("skill", "error", fmt.Sprintf("Skill %s failed: %v", skillName, err))
		writeJSON(w, http.StatusOK, SkillExecuteResponse{
			SkillName: skillName,
			Duration:  duration,
			Error:     err.Error(),
		})
		return
	}

	s.recordEvent("skill", "success", fmt.Sprintf("Executed skill: %s", skillName))
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

	s.recordEvent("api", "info", fmt.Sprintf("Webhook received: %s", webhookPath))
	
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

	s.eventsMu.RLock()
	recentEvents := make([]ActivityEvent, len(s.events))
	copy(recentEvents, s.events)
	s.eventsMu.RUnlock()

	// Reverse events for display (newest first)
	for i, j := 0, len(recentEvents)-1; i < j; i, j = i+1, j-1 {
		recentEvents[i], recentEvents[j] = recentEvents[j], recentEvents[i]
	}

	activeCount := 0
	if manager := s.agentLoop.GetSwarmManager(); manager != nil {
		activeCount = manager.CountRunning()
	}

	// Add 1 for the Primary Agent if the loop is running
	if s.agentLoop.IsRunning() {
		activeCount++
	}

	modelName := "Default"
	if m, ok := startupInfo["model"].(string); ok {
		modelName = m
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	memUsage := fmt.Sprintf("%.2f MB", float64(m.Alloc)/1024/1024)
	numThreads, _ := runtime.ThreadCreateProfile(nil)

	// Get Telemetry
	sessionKey := r.URL.Query().Get("session_key")
	last, sessAvg, overallAvg := s.agentLoop.GetTelemetry(sessionKey)

	writeJSON(w, http.StatusOK, StatusResponse{
		Status:    "ok",
		Version:   s.version,
		Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
		StartedAt: s.startedAt,
		Agent: AgentStatus{
			Model:       modelName,
			ToolsLoaded: toolsInfo["count"].(int),
		},
		Skills: SkillsStatus{
			Total:     skillsInfo["total"].(int),
			Available: skillsInfo["available"].(int),
			Names:     skillNames,
		},
		ActiveAgents: activeCount,
		RecentEvents: recentEvents,
		System: SystemStats{
			MemoryUsage: memUsage,
			CPULoad:     0.5, // Mock value as real CPU load requires external libs/syscalls
			Goroutines:  runtime.NumGoroutine(),
			Threads:     numThreads,
			HeapObjects: m.HeapObjects,
		},
		Workspace: s.getWorkspaceStats(),
		Telemetry: &TelemetryInfo{
			LastResponse:    last,
			SessionAverages: sessAvg,
			OverallAverages: overallAvg,
		},
	})
}

func (s *Server) getWorkspaceStats() WorkspaceStats {
	var totalSize int64
	var count int
	
	_ = filepath.Walk(s.workspace, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		totalSize += info.Size()
		return nil
	})

	sizeStr := fmt.Sprintf("%.2f MB", float64(totalSize)/1024/1024)
	if totalSize < 1024*1024 {
		sizeStr = fmt.Sprintf("%.2f KB", float64(totalSize)/1024)
	}

	return WorkspaceStats{
		TotalFiles: count,
		Size:       sizeStr,
	}
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
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	manager := s.agentLoop.GetSwarmManager()
	if manager == nil {
		writeError(w, http.StatusServiceUnavailable, "swarm_unavailable", "swarm manager not initialized")
		return
	}

	agents := manager.ListAgents()
	
	// Prepend Primary Agent if loop is running
	if s.agentLoop.IsRunning() {
		primaryAgent := map[string]interface{}{
			"id":             "CORE_0000",
			"task":           "RDxClaw Kernel Process",
			"label":          "PrimaryAgent",
			"status":         "RUNNING",
			"created":        s.startedAt.UnixMilli(),
			"origin_channel": "system",
			"is_primary":     true,
		}
		
		// Convert existing agents to []interface{} to easily prepend
		var combined []interface{}
		combined = append(combined, primaryAgent)
		for _, a := range agents {
			combined = append(combined, a)
		}
		
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"agents": combined,
			"count":  len(combined),
		})
		return
	}

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

func (s *Server) handleGetFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "path is required")
		return
	}

	// Security: Ensure path is within the allowed directories
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", fmt.Sprintf("failed to get absolute path: %v", err))
		return
	}

	allowed := false
	docsDirs := []string{s.workspace, filepath.Join(s.workspace, "memory")}
	for _, dir := range docsDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			slog.Warn("Error getting absolute path for docs directory", "dir", dir, "err", err)
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}

	if !allowed {
		writeError(w, http.StatusForbidden, "access_denied", "path is outside allowed documentation directories")
		return
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file_not_found", fmt.Sprintf("failed to read file: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, FileContentResponse{
		Name:    filepath.Base(cleanPath),
		Path:    cleanPath,
		Content: string(content),
	})
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	var files []FileListItem

	err := filepath.Walk(s.workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Include markdown files for the documentation explorer
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(s.workspace, path)
		if err != nil {
			return nil
		}

		files = append(files, FileListItem{
			Name:    info.Name(),
			Path:    path,
			RelPath: relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})

	if err != nil {
		writeError(w, http.StatusInternalServerError, "walk_error", fmt.Sprintf("Failed to list files: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"files": files,
		"count": len(files),
	})
}

func (s *Server) handleUpdateFileContent(w http.ResponseWriter, r *http.Request) {
	var req UpdateFileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", "Failed to decode request body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "path is required")
		return
	}

	// Security: Ensure path is within the allowed directories
	cleanPath := filepath.Clean(req.Path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve absolute path")
		return
	}

	allowed := false
	docsDirs := []string{s.workspace, filepath.Join(s.workspace, "memory")}
	for _, dir := range docsDirs {
		absDir, _ := filepath.Abs(dir)
		if strings.HasPrefix(absPath, absDir) {
			allowed = true
			break
		}
	}

	if !allowed {
		writeError(w, http.StatusForbidden, "access_denied", "Access to this path is restricted")
		return
	}

	// Write content to file
	err = os.WriteFile(cleanPath, []byte(req.Content), 0600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "write_error", fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	s.recordEvent("docs", "info", fmt.Sprintf("Updated file: %s", filepath.Base(cleanPath)))
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sm := s.agentLoop.GetSessionManager()
	if sm == nil {
		writeError(w, http.StatusServiceUnavailable, "session_manager_unavailable", "session manager not initialized")
		return
	}

	sessions := sm.ListSessions()
	writeJSON(w, http.StatusOK, SessionListResponse{
		Sessions: sessions,
		Count:    len(sessions),
	})
}

func (s *Server) handleResumeSession(w http.ResponseWriter, r *http.Request) {
	var req ResumeSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if req.SessionKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "session_key is required")
		return
	}

	// Use a non-request context for the background execution
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	// Don't defer cancel() here as we want the goroutine to finish

	// Parse origin channel from session key if possible
	channel := "api"
	chatID := "api"
	if idx := strings.Index(req.SessionKey, ":"); idx > 0 {
		channel = req.SessionKey[:idx]
		chatID = req.SessionKey[idx+1:]
	}

	s.recordEvent("agent", "info", fmt.Sprintf("Resuming session: %s", req.SessionKey))

	// Resuming is just calling ProcessDirect with an empty user message.
	go func() {
		defer cancel() // Cancel when the goroutine finishes
		_, err := s.agentLoop.ProcessDirectWithChannel(ctx, "", req.SessionKey, channel, chatID)
		if err != nil {
			s.recordEvent("agent", "error", fmt.Sprintf("Resume error: %v", err))
		} else {
			s.recordEvent("agent", "success", fmt.Sprintf("Session %s completed after resume", req.SessionKey))
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"success": true,
		"message": "Resume triggered in background",
	})
}

func (s *Server) handleGetRecoveryConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, RecoveryConfig{
		AutoResume: true,
	})
}

func (s *Server) handleUpdateRecoveryConfig(w http.ResponseWriter, r *http.Request) {
	var req RecoveryConfig
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	
	s.recordEvent("system", "info", fmt.Sprintf("Updated recovery config: AutoResume=%v", req.AutoResume))
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	clientChan := make(chan string, 20)
	s.clientsMu.Lock()
	s.clients[clientChan] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientChan)
		s.clientsMu.Unlock()
	}()

	// Keep alive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Send initial message to establish connection
	fmt.Fprintf(w, "event: connected\ndata: {\"timestamp\":%d}\n\n", time.Now().UnixMilli())
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case msg := <-clientChan:
			fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keep-alive\n\n")
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) statusBroadcaster() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Only broadcast if there are active clients
		s.clientsMu.Lock()
		hasClients := len(s.clients) > 0
		s.clientsMu.Unlock()

		if !hasClients {
			continue
		}

		// Re-use logic from handleStatus but simplified for broadcast
		startupInfo := s.agentLoop.GetStartupInfo()
		
		allSkills := s.loader.ListSkills()
		skillNames := make([]string, len(allSkills))
		for i, skill := range allSkills {
			skillNames[i] = skill.Name
		}

		activeCount := 0
		if manager := s.agentLoop.GetSwarmManager(); manager != nil {
			activeCount = manager.CountRunning()
		}
		if s.agentLoop.IsRunning() {
			activeCount++
		}

		modelName := "Default"
		if m, ok := startupInfo["model"].(string); ok {
			modelName = m
		}

		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memUsage := fmt.Sprintf("%.2f MB", float64(m.Alloc)/1024/1024)
		numThreads, _ := runtime.ThreadCreateProfile(nil)

		status := StatusResponse{
			Status:    "ok",
			Version:   s.version,
			Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
			StartedAt: s.startedAt,
			Agent: AgentStatus{
				Model:       modelName,
				ToolsLoaded: startupInfo["tools"].(map[string]interface{})["count"].(int),
			},
			Skills: SkillsStatus{
				Total:     len(allSkills),
				Available: len(allSkills),
				Names:     skillNames,
			},
			ActiveAgents: activeCount,
			RecentEvents: []ActivityEvent{}, // Avoid sending huge history in broadcast
			System: SystemStats{
				MemoryUsage: memUsage,
				CPULoad:     0.5,
				Goroutines:  runtime.NumGoroutine(),
				Threads:     numThreads,
				HeapObjects: m.HeapObjects,
			},
			Workspace: s.getWorkspaceStats(),
		}

		// Include telemetry so the dashboard latency panel updates in real-time via SSE
		last, sessAvg, overallAvg := s.agentLoop.GetTelemetry("")
		status.Telemetry = &TelemetryInfo{
			LastResponse:    last,
			SessionAverages: sessAvg,
			OverallAverages: overallAvg,
		}

		s.broadcast("status", status)
	}
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
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
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
