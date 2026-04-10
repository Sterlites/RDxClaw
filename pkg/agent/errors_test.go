package agent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestErrorClassification verifies classifyLLMError correctly categorizes error strings.
func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorKind
	}{
		{"nil error", nil, ErrUnknown},
		{"context canceled", context.Canceled, ErrTimeout},
		{"deadline exceeded", context.DeadlineExceeded, ErrTimeout},

		// Context overflow
		{"token limit", fmt.Errorf("max token limit exceeded"), ErrContextOverflow},
		{"context length", fmt.Errorf("context length exceeded: too many tokens"), ErrContextOverflow},
		{"invalidparameter tokens", fmt.Errorf("InvalidParameter: Total tokens of image and text exceed max message tokens"), ErrContextOverflow},
		{"prompt too long", fmt.Errorf("prompt is too long for this model"), ErrContextOverflow},

		// Rate limit
		{"rate limit", fmt.Errorf("rate limit exceeded"), ErrRateLimit},
		{"429 status", fmt.Errorf("API returned 429: too many requests"), ErrRateLimit},
		{"quota exceeded", fmt.Errorf("quota exceeded for this billing period"), ErrQuotaExceeded},
		{"throttled", fmt.Errorf("request was throttled"), ErrRateLimit},

		// Timeout
		{"timeout", fmt.Errorf("request timeout after 30s"), ErrTimeout},
		{"timed out", fmt.Errorf("connection timed out"), ErrTimeout},
		{"504 gateway timeout", fmt.Errorf("504 gateway timeout"), ErrTimeout},

		// Auth
		{"unauthorized", fmt.Errorf("unauthorized: invalid API key"), ErrAuth},
		{"401 status", fmt.Errorf("HTTP 401: authentication required"), ErrAuth},
		{"403 forbidden", fmt.Errorf("403 forbidden: access denied"), ErrAuth},

		// Transient
		{"overloaded", fmt.Errorf("server is overloaded, try again later"), ErrTransient},
		{"503 unavailable", fmt.Errorf("503 service unavailable"), ErrTransient},
		{"502 bad gateway", fmt.Errorf("502 bad gateway"), ErrTransient},
		{"internal server error", fmt.Errorf("internal server error"), ErrTransient},

		// Fatal
		{"bad request", fmt.Errorf("bad request: invalid parameters"), ErrFatal},
		{"model not found", fmt.Errorf("model not found: invalid-model"), ErrFatal},

		// Unknown
		{"random error", fmt.Errorf("something random happened"), ErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyLLMError(tt.err)
			if got != tt.expected {
				t.Errorf("classifyLLMError(%v) = %s, want %s", tt.err, got.String(), tt.expected.String())
			}
		})
	}
}

// TestIsRetryable verifies the retryability map for each error kind.
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		kind     ErrorKind
		expected bool
	}{
		{ErrContextOverflow, true},
		{ErrRateLimit, true},
		{ErrTransient, true},
		{ErrTimeout, true},
		{ErrAuth, false},
		{ErrFatal, false},
		{ErrUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			got := isRetryable(tt.kind)
			if got != tt.expected {
				t.Errorf("isRetryable(%s) = %v, want %v", tt.kind.String(), got, tt.expected)
			}
		})
	}
}

// TestComputeBackoff verifies exponential backoff produces correct delays.
func TestComputeBackoff(t *testing.T) {
	policy := BackoffPolicy{
		InitialMs: 500,
		MaxMs:     8000,
		Factor:    2.0,
		Jitter:    0.0, // No jitter for deterministic test
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 500 * time.Millisecond},
		{2, 1000 * time.Millisecond},
		{3, 2000 * time.Millisecond},
		{4, 4000 * time.Millisecond},
		{5, 8000 * time.Millisecond}, // Capped at MaxMs
		{6, 8000 * time.Millisecond}, // Still capped
		{0, 500 * time.Millisecond},  // Edge case: attempt 0 treated as 1
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			got := computeBackoff(policy, tt.attempt)
			if got != tt.expected {
				t.Errorf("computeBackoff(attempt=%d) = %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}

	// Test with jitter: result should be within expected range
	t.Run("with_jitter", func(t *testing.T) {
		jitterPolicy := BackoffPolicy{
			InitialMs: 1000,
			MaxMs:     8000,
			Factor:    2.0,
			Jitter:    0.2,
		}
		for i := 0; i < 100; i++ {
			got := computeBackoff(jitterPolicy, 1)
			// Base = 1000ms, jitter = ±20% → range [800ms, 1200ms]
			if got < 800*time.Millisecond || got > 1200*time.Millisecond {
				t.Errorf("computeBackoff with jitter = %v, expected [800ms, 1200ms]", got)
			}
		}
	})
}

// TestSleepWithContext_Cancellation verifies sleep respects context cancellation.
func TestSleepWithContext_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	start := time.Now()
	err := sleepWithContext(ctx, 10*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected context error, got nil")
	}

	// Should return almost immediately (well under 1 second)
	if elapsed > 500*time.Millisecond {
		t.Errorf("sleepWithContext took %v, expected near-instant cancellation", elapsed)
	}
}

// TestSleepWithContext_ZeroDuration verifies zero duration returns immediately.
func TestSleepWithContext_ZeroDuration(t *testing.T) {
	err := sleepWithContext(context.Background(), 0)
	if err != nil {
		t.Errorf("expected nil for zero duration, got %v", err)
	}
}

// TestToolResultTruncation verifies truncateToolResult works correctly.
func TestToolResultTruncation(t *testing.T) {
	// Short content should not be truncated
	t.Run("short_content", func(t *testing.T) {
		content := "Hello, world!"
		result, truncated := truncateToolResult(content)
		if truncated {
			t.Error("expected no truncation for short content")
		}
		if result != content {
			t.Errorf("expected unchanged content, got %q", result)
		}
	})

	// Content at exactly the limit should not be truncated
	t.Run("at_limit", func(t *testing.T) {
		content := make([]byte, MaxToolResultBytes)
		for i := range content {
			content[i] = 'a'
		}
		result, truncated := truncateToolResult(string(content))
		if truncated {
			t.Error("expected no truncation at exact limit")
		}
		if len(result) != MaxToolResultBytes {
			t.Errorf("expected length %d, got %d", MaxToolResultBytes, len(result))
		}
	})

	// Content over the limit should be truncated
	t.Run("over_limit", func(t *testing.T) {
		content := make([]byte, MaxToolResultBytes+1000)
		for i := range content {
			content[i] = 'b'
		}
		result, truncated := truncateToolResult(string(content))
		if !truncated {
			t.Error("expected truncation for oversized content")
		}
		if len(result) > MaxToolResultBytes+100 { // Some room for the truncation marker
			t.Errorf("expected truncated result, got length %d", len(result))
		}
		// Should contain the truncation marker
		if len(result) > 0 {
			expected := "...[TRUNCATED: output exceeded 100KB limit]..."
			if result[len(result)-len(expected):] != expected {
				t.Errorf("expected truncation marker at end")
			}
		}
	})
}

// TestUsageAccumulator verifies usage tracking across multiple merges.
func TestUsageAccumulator(t *testing.T) {
	u := &UsageAccumulator{}

	u.Merge(100, 50, 150)
	if u.InputTokens != 100 || u.OutputTokens != 50 || u.TotalTokens != 150 {
		t.Errorf("after first merge: input=%d output=%d total=%d", u.InputTokens, u.OutputTokens, u.TotalTokens)
	}
	if u.CallCount != 1 {
		t.Errorf("expected CallCount=1, got %d", u.CallCount)
	}
	if u.LastCallInput != 100 || u.LastCallOutput != 50 || u.LastCallTotal != 150 {
		t.Error("last call values should match first merge")
	}

	u.Merge(200, 80, 280)
	if u.InputTokens != 300 || u.OutputTokens != 130 || u.TotalTokens != 430 {
		t.Errorf("after second merge: input=%d output=%d total=%d", u.InputTokens, u.OutputTokens, u.TotalTokens)
	}
	if u.CallCount != 2 {
		t.Errorf("expected CallCount=2, got %d", u.CallCount)
	}
	// Last call should reflect the MOST RECENT call, not accumulated
	if u.LastCallInput != 200 || u.LastCallOutput != 80 || u.LastCallTotal != 280 {
		t.Error("last call values should match second merge, not accumulated")
	}
}

// TestErrorKindString verifies String() returns expected labels.
func TestErrorKindString(t *testing.T) {
	tests := []struct {
		kind     ErrorKind
		expected string
	}{
		{ErrContextOverflow, "context_overflow"},
		{ErrRateLimit, "rate_limit"},
		{ErrTimeout, "timeout"},
		{ErrAuth, "auth"},
		{ErrTransient, "transient"},
		{ErrFatal, "fatal"},
		{ErrUnknown, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.expected {
			t.Errorf("%d.String() = %q, want %q", tt.kind, got, tt.expected)
		}
	}
}
