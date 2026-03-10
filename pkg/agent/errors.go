// RDxClaw - High-performance Agentic AI Framework for the Edge
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 rdxclaw contributors

package agent

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"strings"
	"time"
)

// ErrorKind classifies LLM errors into recovery categories.
// RDxClaw's classification system for production-grade error handling.
type ErrorKind int

const (
	// ErrUnknown is an unclassified error (treated as fatal by default).
	ErrUnknown ErrorKind = iota
	// ErrContextOverflow means the prompt/context exceeds the model's token limit.
	ErrContextOverflow
	// ErrRateLimit means the provider is throttling requests.
	ErrRateLimit
	// ErrTimeout means the request timed out (network or model-side).
	ErrTimeout
	// ErrAuth means authentication/authorization failed.
	ErrAuth
	// ErrTransient is a temporary server-side error (overloaded, 5xx, etc).
	ErrTransient
	// ErrFatal is an unrecoverable error (bad request, invalid model, etc).
	ErrFatal
)

// String returns a human-readable label for the error kind.
func (k ErrorKind) String() string {
	switch k {
	case ErrContextOverflow:
		return "context_overflow"
	case ErrRateLimit:
		return "rate_limit"
	case ErrTimeout:
		return "timeout"
	case ErrAuth:
		return "auth"
	case ErrTransient:
		return "transient"
	case ErrFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// classifyLLMError examines an error from an LLM provider call and classifies it
// into an ErrorKind for routing to the appropriate recovery path.
//
// This replaces fragile inline string-matching scattered across the loop with a
// single, centralised classification function defined by RDxClaw's
// standard error handling patterns.
func classifyLLMError(err error) ErrorKind {
	if err == nil {
		return ErrUnknown
	}

	// Context cancellation / deadline exceeded
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}

	msg := strings.ToLower(err.Error())

	// --- Context overflow ---
	// Match patterns like "token limit", "context length exceeded",
	// "max.*token", "InvalidParameter...tokens", etc.
	contextOverflowPatterns := []string{
		"token",
		"context length",
		"context_length",
		"context window",
		"maximum context",
		"invalidparameter",
		"prompt is too long",
		"request too large",
		"input too long",
		"max_tokens",
		"content_too_large",
	}
	for _, p := range contextOverflowPatterns {
		if strings.Contains(msg, p) {
			return ErrContextOverflow
		}
	}

	// --- Rate limiting ---
	rateLimitPatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
		"quota exceeded",
		"throttl",
	}
	for _, p := range rateLimitPatterns {
		if strings.Contains(msg, p) {
			return ErrRateLimit
		}
	}

	// --- Timeout ---
	timeoutPatterns := []string{
		"timeout",
		"timed out",
		"deadline exceeded",
		"gateway timeout",
		"504",
		"408",
	}
	for _, p := range timeoutPatterns {
		if strings.Contains(msg, p) {
			return ErrTimeout
		}
	}

	// --- Auth ---
	authPatterns := []string{
		"unauthorized",
		"401",
		"403",
		"forbidden",
		"invalid api key",
		"invalid_api_key",
		"authentication",
		"permission denied",
	}
	for _, p := range authPatterns {
		if strings.Contains(msg, p) {
			return ErrAuth
		}
	}

	// --- Transient (server errors, overload) ---
	transientPatterns := []string{
		"overloaded",
		"503",
		"502",
		"500",
		"internal server error",
		"service unavailable",
		"bad gateway",
		"temporarily unavailable",
		"server error",
		"capacity",
	}
	for _, p := range transientPatterns {
		if strings.Contains(msg, p) {
			return ErrTransient
		}
	}

	// --- Fatal (bad request, model not found) ---
	fatalPatterns := []string{
		"400",
		"bad request",
		"invalid model",
		"model not found",
		"not found",
		"unsupported",
	}
	for _, p := range fatalPatterns {
		if strings.Contains(msg, p) {
			return ErrFatal
		}
	}

	return ErrUnknown
}

// isRetryable returns true if the error kind warrants a retry.
func isRetryable(kind ErrorKind) bool {
	switch kind {
	case ErrContextOverflow, ErrRateLimit, ErrTransient, ErrTimeout:
		return true
	default:
		return false
	}
}

// BackoffPolicy configures exponential backoff behaviour.
type BackoffPolicy struct {
	InitialMs int     // Initial delay in milliseconds (default: 500)
	MaxMs     int     // Maximum delay in milliseconds (default: 8000)
	Factor    float64 // Multiplier per attempt (default: 2.0)
	Jitter    float64 // Jitter fraction 0–1 (default: 0.2)
}

// DefaultBackoffPolicy is the standard backoff for transient/rate-limit retries.
var DefaultBackoffPolicy = BackoffPolicy{
	InitialMs: 500,
	MaxMs:     8000,
	Factor:    2.0,
	Jitter:    0.2,
}

// computeBackoff calculates the backoff delay for a given attempt number (1-based).
// Returns a duration incorporating exponential growth, cap, and random jitter.
func computeBackoff(policy BackoffPolicy, attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	base := float64(policy.InitialMs) * math.Pow(policy.Factor, float64(attempt-1))
	if base > float64(policy.MaxMs) {
		base = float64(policy.MaxMs)
	}

	// Apply jitter: base ± (jitter * base)
	jitterRange := base * policy.Jitter
	jittered := base + jitterRange*(2*rand.Float64()-1) //nolint:gosec // jitter doesn't need crypto rand

	if jittered < 0 {
		jittered = 0
	}

	return time.Duration(jittered) * time.Millisecond
}

// sleepWithContext sleeps for the given duration, but returns early if the
// context is cancelled. Returns ctx.Err() if cancelled, nil otherwise.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// UsageAccumulator tracks cumulative token usage across all LLM calls within
// a single request turn. RDxClaw's UsageAccumulator 
// prevents inflated context-size reporting by separating accumulated totals
// from per-call snapshots.
type UsageAccumulator struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int

	// Per-call snapshots from the most recent API call, used for accurate
	// context-size reporting (accumulated totals inflate with each round-trip).
	LastCallInput  int
	LastCallOutput int
	LastCallTotal  int

	// CallCount tracks how many LLM API calls have been made this turn.
	CallCount int
}

// Merge adds usage from a single LLM API call into the accumulator.
func (u *UsageAccumulator) Merge(input, output, total int) {
	u.InputTokens += input
	u.OutputTokens += output
	u.TotalTokens += total
	u.LastCallInput = input
	u.LastCallOutput = output
	u.LastCallTotal = total
	u.CallCount++
}

// MaxToolResultBytes is the maximum size of a single tool result before truncation.
// Tool results exceeding this threshold are truncated to prevent blowing the
// context window on the next LLM call.
const MaxToolResultBytes = 100 * 1024 // 100KB

// truncateToolResult truncates a tool result string if it exceeds MaxToolResultBytes.
// Returns the (possibly truncated) string and whether truncation occurred.
func truncateToolResult(content string) (string, bool) {
	if len(content) <= MaxToolResultBytes {
		return content, false
	}
	// Keep the first portion and append a truncation marker
	return content[:MaxToolResultBytes] + "\n...[TRUNCATED: output exceeded 100KB limit]...", true
}

// MaxOverflowCompactionAttempts is the maximum number of context overflow
// recovery cycles (compaction + truncation) before giving up.
const MaxOverflowCompactionAttempts = 3

// MaxConsecutiveNudges is the maximum number of preamble/empty-response nudges
// before accepting whatever content exists.
const MaxConsecutiveNudges = 2
