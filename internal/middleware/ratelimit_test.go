package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRateLimit_UnderLimit(t *testing.T) {
	rl := &RateLimiter{
		rate:    1.0,
		burst:   10,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		buckets: make(map[string]*bucket),
	}
	handler := rl.Limit(okHandler)

	for i := range 4 {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), userCtxKey, "user-1")
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d expected 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimit_Exceeded(t *testing.T) {
	rl := &RateLimiter{
		rate:    0.0,
		burst:   0,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		buckets: make(map[string]*bucket),
	}
	handler := rl.Limit(okHandler)

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), userCtxKey, "user-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestRateLimit_PerUserIsolation(t *testing.T) {
	rl := &RateLimiter{
		rate:    0.0,
		burst:   1,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		buckets: make(map[string]*bucket),
	}
	handler := rl.Limit(okHandler)

	reqA1 := httptest.NewRequest("GET", "/", nil)
	reqA2 := httptest.NewRequest("GET", "/", nil)
	reqB := httptest.NewRequest("GET", "/", nil)
	ctxA1 := context.WithValue(reqA1.Context(), userCtxKey, "user-1")
	ctxA2 := context.WithValue(reqA2.Context(), userCtxKey, "user-1")
	ctxB := context.WithValue(reqB.Context(), userCtxKey, "user-2")
	reqA1 = reqA1.WithContext(ctxA1)
	reqA2 = reqA2.WithContext(ctxA2)
	reqB = reqB.WithContext(ctxB)
	wA1 := httptest.NewRecorder()
	wA2 := httptest.NewRecorder()
	wB := httptest.NewRecorder()
	handler(wA1, reqA1)
	handler(wA2, reqA2)
	handler(wB, reqB)
	if wA1.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", wA1.Code)
	}
	if wA2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", wA2.Code)
	}
	if wB.Code != http.StatusOK {
		t.Errorf("expected 200 (diff bucket), got %d", wB.Code)
	}
}

func TestRateLimit_FallBackToIP(t *testing.T) {
	rl := &RateLimiter{
		rate:    0.0,
		burst:   0,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		buckets: make(map[string]*bucket),
	}
	handler := rl.Limit(okHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.2:4321"
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
