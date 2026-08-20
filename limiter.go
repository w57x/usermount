package main

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type clientRecord struct {
	tokens     float64
	lastRefill time.Time
}

type IPRateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientRecord
	rate        float64 // tokens per second
	burst       float64
	lastCleanup time.Time
}

func NewIPRateLimiter(rate float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		clients:     make(map[string]*clientRecord),
		rate:        rate,
		burst:       float64(burst),
		lastCleanup: time.Now(),
	}
}

func (limiter *IPRateLimiter) Allow(ip string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()

	// Clean stale entries every 5 minutes
	if now.Sub(limiter.lastCleanup) > 5*time.Minute {
		for k, v := range limiter.clients {
			if now.Sub(v.lastRefill) > 10*time.Minute {
				delete(limiter.clients, k)
			}
		}
		limiter.lastCleanup = now
	}

	record, exists := limiter.clients[ip]
	if !exists {
		limiter.clients[ip] = &clientRecord{
			tokens:     limiter.burst - 1,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(record.lastRefill).Seconds()
	record.lastRefill = now
	record.tokens += elapsed * limiter.rate
	if record.tokens > limiter.burst {
		record.tokens = limiter.burst
	}

	if record.tokens >= 1.0 {
		record.tokens -= 1.0
		return true
	}

	return false
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For if behind a reverse proxy
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	xRealIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xRealIP != "" {
		return xRealIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func RateLimitMiddleware(limiter *IPRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !limiter.Allow(ip) {
			http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
