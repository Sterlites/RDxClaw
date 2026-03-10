// RDxClaw - High-performance Agentic AI Framework for the Edge
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 rdxclaw contributors

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sterlites/RDxClaw/pkg/logger"
	"github.com/Sterlites/RDxClaw/pkg/providers"
	"github.com/Sterlites/RDxClaw/pkg/utils"
)

// ToolLoopConfig configures the tool execution loop.
type ToolLoopConfig struct {
	Provider      providers.LLMProvider
	Model         string
	Tools         *ToolRegistry
	MaxIterations int
	LLMOptions    map[string]any
}

// ToolLoopResult contains the result of running the tool loop.
type ToolLoopResult struct {
	Content    string
	Iterations int
}

// maxToolResultBytesSubagent is the maximum size of a single tool result in
// the subagent tool loop. Mirrors the main agent's limit to prevent context
// blowout.
const maxToolResultBytesSubagent = 100 * 1024 // 100KB

// RunToolLoop executes the LLM + tool call iteration loop.
// This is the core agent logic that can be reused by both main agent and subagents.
//
// SoTA enhancements (inspired by OpenClaw):
//   - Tool result size guards to prevent context blowout
//   - Context cancellation checks between tool calls
//   - Error classification for smarter retry decisions
//   - Hard loop guard to prevent runaway iterations
func RunToolLoop(ctx context.Context, config ToolLoopConfig, messages []providers.Message, channel, chatID string) (*ToolLoopResult, error) {
	iteration := 0
	var finalContent string

	// SoTA: Hard loop guard
	hardLoopLimit := config.MaxIterations + 4
	totalLoops := 0

	for iteration < config.MaxIterations {
		iteration++
		totalLoops++

		// SoTA: Hard loop guard
		if totalLoops > hardLoopLimit {
			logger.ErrorCF("toolloop", "Hard loop limit exceeded",
				map[string]any{
					"total_loops": totalLoops,
					"hard_limit":  hardLoopLimit,
				})
			if finalContent == "" {
				finalContent = "Subagent request failed after repeated internal retries."
			}
			break
		}

		// SoTA: Check context cancellation before each iteration
		select {
		case <-ctx.Done():
			logger.WarnCF("toolloop", "Context cancelled between iterations",
				map[string]any{"iteration": iteration})
			return &ToolLoopResult{
				Content:    finalContent,
				Iterations: iteration,
			}, ctx.Err()
		default:
		}

		logger.DebugCF("toolloop", "LLM iteration",
			map[string]any{
				"iteration":   iteration,
				"max":         config.MaxIterations,
				"total_loops": totalLoops,
			})

		// 1. Build tool definitions
		var providerToolDefs []providers.ToolDefinition
		if config.Tools != nil {
			providerToolDefs = config.Tools.ToProviderDefs()
		}

		// 2. Set default LLM options
		llmOpts := config.LLMOptions
		if llmOpts == nil {
			llmOpts = map[string]any{
				"max_tokens":  4096,
				"temperature": 0.7,
			}
		}

		// 3. Call LLM with basic retry for transient errors
		var response *providers.LLMResponse
		var err error
		maxRetries := 2
		for retry := 0; retry <= maxRetries; retry++ {
			response, err = config.Provider.Chat(ctx, messages, providerToolDefs, config.Model, llmOpts)
			if err == nil {
				break
			}

			// SoTA: Classify error for smarter retry
			errMsg := strings.ToLower(err.Error())
			isTransient := strings.Contains(errMsg, "503") ||
				strings.Contains(errMsg, "502") ||
				strings.Contains(errMsg, "overloaded") ||
				strings.Contains(errMsg, "rate limit") ||
				strings.Contains(errMsg, "429") ||
				strings.Contains(errMsg, "timeout")

			if isTransient && retry < maxRetries {
				logger.WarnCF("toolloop", "Transient LLM error, retrying",
					map[string]any{
						"error": err.Error(),
						"retry": retry,
					})
				continue
			}

			// Non-transient or exhausted retries
			break
		}

		if err != nil {
			logger.ErrorCF("toolloop", "LLM call failed",
				map[string]any{
					"iteration": iteration,
					"error":     err.Error(),
				})
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// 4. If no tool calls, we're done
		if len(response.ToolCalls) == 0 {
			finalContent = response.Content
			logger.InfoCF("toolloop", "LLM response without tool calls (direct answer)",
				map[string]any{
					"iteration":     iteration,
					"content_chars": len(finalContent),
				})
			break
		}

		// 5. Log tool calls
		toolNames := make([]string, 0, len(response.ToolCalls))
		for _, tc := range response.ToolCalls {
			toolNames = append(toolNames, tc.Name)
		}
		logger.InfoCF("toolloop", "LLM requested tool calls",
			map[string]any{
				"tools":     toolNames,
				"count":     len(response.ToolCalls),
				"iteration": iteration,
			})

		// 6. Build assistant message with tool calls
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

		// 7. Execute tool calls
		for _, tc := range response.ToolCalls {
			// SoTA: Check context cancellation between tool calls
			select {
			case <-ctx.Done():
				logger.WarnCF("toolloop", "Context cancelled between tool calls",
					map[string]any{"tool": tc.Name, "iteration": iteration})
				return &ToolLoopResult{
					Content:    finalContent,
					Iterations: iteration,
				}, ctx.Err()
			default:
			}

			argsJSON, _ := json.Marshal(tc.Arguments)
			argsPreview := utils.Truncate(string(argsJSON), 200)
			logger.InfoCF("toolloop", fmt.Sprintf("Tool call: %s(%s)", tc.Name, argsPreview),
				map[string]any{
					"tool":      tc.Name,
					"iteration": iteration,
				})

			// Execute tool
			var toolResult *ToolResult
			if config.Tools != nil {
				toolResult = config.Tools.ExecuteWithContext(ctx, tc.Name, tc.Arguments, channel, chatID, nil)
			} else {
				toolResult = ErrorResult("No tools available")
			}

			// Determine content for LLM
			contentForLLM := toolResult.ForLLM
			if contentForLLM == "" && toolResult.Err != nil {
				contentForLLM = toolResult.Err.Error()
			}

			// SoTA: Tool result size guard — truncate oversized results
			if len(contentForLLM) > maxToolResultBytesSubagent {
				logger.WarnCF("toolloop", "Tool result truncated (exceeds size limit)",
					map[string]any{
						"tool":          tc.Name,
						"original_size": len(contentForLLM),
						"max_size":      maxToolResultBytesSubagent,
					})
				contentForLLM = contentForLLM[:maxToolResultBytesSubagent] +
					"\n...[TRUNCATED: output exceeded 100KB limit]..."
			}

			// Add tool result message
			toolResultMsg := providers.Message{
				Role:       "tool",
				Content:    contentForLLM,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolResultMsg)
		}
	}

	return &ToolLoopResult{
		Content:    finalContent,
		Iterations: iteration,
	}, nil
}
