package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	if rl == nil {
		t.Fatal("NewRateLimiter() = nil")
	}
	if rl.limit != 10 {
		t.Errorf("limit = %d, want 10", rl.limit)
	}
	if rl.window != time.Minute {
		t.Errorf("window = %v, want %v", rl.window, time.Minute)
	}
}

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	if !rl.Allow("192.168.1.1") {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_Allow_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)
	ip := "192.168.1.2"

	for i := 0; i < 10; i++ {
		if !rl.Allow(ip) {
			t.Errorf("request %d should be allowed (under limit)", i+1)
		}
	}
}

func TestRateLimiter_Allow_OverLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	ip := "192.168.1.3"

	// Use up the limit
	for i := 0; i < 5; i++ {
		rl.Allow(ip)
	}

	// Next request should be blocked
	if rl.Allow(ip) {
		t.Error("request over limit should be blocked")
	}
}

func TestRateLimiter_Allow_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// IP 1 uses its limit
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Error("10.0.0.1 should be blocked after 2 requests")
	}

	// IP 2 should still work
	if !rl.Allow("10.0.0.2") {
		t.Error("10.0.0.2 should be allowed")
	}
}

func TestRateLimiter_Allow_WindowReset(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	ip := "192.168.1.4"

	// Use up the limit
	rl.Allow(ip)
	rl.Allow(ip)
	if rl.Allow(ip) {
		t.Error("should be blocked after limit")
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow(ip) {
		t.Error("should be allowed after window reset")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 50*time.Millisecond)

	rl.Allow("cleanup.test.1")
	rl.Allow("cleanup.test.2")

	// Wait for entries to expire
	time.Sleep(60 * time.Millisecond)

	// Trigger cleanup manually
	rl.cleanup()

	rl.mu.Lock()
	count := len(rl.visitors)
	rl.mu.Unlock()

	if count != 0 {
		t.Errorf("visitors count after cleanup = %d, want 0", count)
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrapped := rl.Middleware(handler)

	// First two requests should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// Third request should be blocked
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("blocked request: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Middleware_XForwardedFor(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// First request with X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	req1.RemoteAddr = "10.0.0.1:12345" // Different RemoteAddr
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	// Second request from same X-Forwarded-For
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Forwarded-For", "203.0.113.1")
	req2.RemoteAddr = "10.0.0.2:12345" // Different RemoteAddr
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Error("should use X-Forwarded-For for rate limiting")
	}
}

func TestRateLimiter_Middleware_FallbackToRemoteAddr(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := rl.Middleware(handler)

	// No X-Forwarded-For
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "192.168.1.100:5000"
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "192.168.1.100:5000" // Same RemoteAddr (middleware uses full addr including port)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Error("should fall back to RemoteAddr when X-Forwarded-For is empty")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, time.Minute)

	done := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				rl.Allow("concurrent.test")
			}
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	// Should not panic or deadlock
}
