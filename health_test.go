package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Test /healthz
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	rrHealth := httptest.NewRecorder()
	mux.ServeHTTP(rrHealth, reqHealth)

	if rrHealth.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /healthz, got %d", rrHealth.Code)
	}
	if rrHealth.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got '%s'", rrHealth.Body.String())
	}

	// Test /livez
	reqLive := httptest.NewRequest("GET", "/livez", nil)
	rrLive := httptest.NewRecorder()
	mux.ServeHTTP(rrLive, reqLive)

	if rrLive.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /livez, got %d", rrLive.Code)
	}
	if rrLive.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got '%s'", rrLive.Body.String())
	}
}
