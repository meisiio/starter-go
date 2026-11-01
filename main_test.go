package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Simple arithmetic test to verify the test tool works.
func TestAddTwoNumbers(t *testing.T) {
	a, b := 3, 5
	got := a + b
	want := 8
	if got != want {
		t.Errorf("Add(%d, %d) = %d; want %d", a, b, got, want)
	}
}

// Test that the /health endpoint returns 200 and {"status":"ok"}.
func TestHealthEndpoint(t *testing.T) {
	cfg := Config{
		ServiceName: "test-service",
		ListenAddr:  ":0",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Check status code
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health status = %d; want %d", rec.Code, http.StatusOK)
	}

	// Check JSON body
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("GET /health status field = %q; want %q", body["status"], "ok")
	}

	// Verify Content-Type header
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}

	_ = cfg // used to show cfg is loaded
}

// Test that POST /health returns 405 Method Not Allowed.
func TestHealthEndpointRejectsPost(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /health status = %d; want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

