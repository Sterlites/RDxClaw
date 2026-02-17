package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allows request with valid key", func(t *testing.T) {
		middleware := AuthMiddleware("test-key", handler)
		req := httptest.NewRequest("GET", "/v1/skills", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("rejects request with invalid key", func(t *testing.T) {
		middleware := AuthMiddleware("test-key", handler)
		req := httptest.NewRequest("GET", "/v1/skills", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("rejects request without auth header", func(t *testing.T) {
		middleware := AuthMiddleware("test-key", handler)
		req := httptest.NewRequest("GET", "/v1/skills", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("allows unauthenticated request when no key configured", func(t *testing.T) {
		middleware := AuthMiddleware("", handler)
		req := httptest.NewRequest("GET", "/v1/skills", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("health endpoint bypasses auth", func(t *testing.T) {
		middleware := AuthMiddleware("test-key", handler)
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("status endpoint bypasses auth", func(t *testing.T) {
		middleware := AuthMiddleware("test-key", handler)
		req := httptest.NewRequest("GET", "/v1/status", nil)
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewRateLimiter(3, time.Second)
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.1"))
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Second)
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.False(t, rl.Allow("192.168.1.1"))
	})

	t.Run("different IPs have independent limits", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Second)
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.2"))
		assert.False(t, rl.Allow("192.168.1.1"))
	})

	t.Run("resets after window", func(t *testing.T) {
		rl := NewRateLimiter(1, 50*time.Millisecond)
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.False(t, rl.Allow("192.168.1.1"))
		time.Sleep(60 * time.Millisecond)
		assert.True(t, rl.Allow("192.168.1.1"))
	})
}

func TestCORSMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("sets CORS headers for allowed origin", func(t *testing.T) {
		middleware := CORSMiddleware([]string{"http://example.com"}, handler)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, "http://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("handles OPTIONS preflight", func(t *testing.T) {
		middleware := CORSMiddleware([]string{"*"}, handler)
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "http://example.com")
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("wildcard allows any origin", func(t *testing.T) {
		middleware := CORSMiddleware([]string{"*"}, handler)
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Origin", "http://anything.com")
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, req)
		assert.Equal(t, "http://anything.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}
