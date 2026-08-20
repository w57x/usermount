package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	limiter := NewIPRateLimiter(1.0, 3) // 3 burst, 1 token/sec

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("127.0.0.1") {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}

	// 4th immediate request should be denied
	if limiter.Allow("127.0.0.1") {
		t.Fatalf("expected 4th immediate request to be denied")
	}

	// Different IP should still be allowed
	if !limiter.Allow("192.168.1.1") {
		t.Fatalf("expected request from different IP to be allowed")
	}

	// Wait for token refill
	time.Sleep(1100 * time.Millisecond)
	if !limiter.Allow("127.0.0.1") {
		t.Fatalf("expected refilled request to be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewIPRateLimiter(1.0, 1)

	handler := RateLimitMiddleware(limiter, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// First request succeeds
	req1 := httptest.NewRequest("POST", "/api/login", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr1.Code)
	}

	// Second immediate request gets 429
	req2 := httptest.NewRequest("POST", "/api/login", nil)
	req2.RemoteAddr = "10.0.0.1:12345"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", rr2.Code)
	}
}
