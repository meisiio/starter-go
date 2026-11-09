package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---- Unit tests (no database, no network) -----------------------------------

func TestUnitExceptionCursorRoundTrip(t *testing.T) {
	want := exceptionCursor{
		CreatedAt: time.Date(2026, 11, 1, 10, 30, 0, 123456000, time.UTC),
		ID:        "0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11",
	}

	got, err := decodeExceptionCursor(want.encode())
	if err != nil {
		t.Fatalf("decode(encode(c)) returned error: %v", err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Errorf("round trip changed the cursor: got %+v, want %+v", got, want)
	}
}

func TestUnitDecodeExceptionCursorRejectsInvalidTokens(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

	cases := map[string]string{
		"not base64":         "%%%",
		"not json":           enc("not json"),
		"empty json object":  enc("{}"),
		"missing created_at": enc(`{"id":"0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11"}`),
		"id is not a uuid":   enc(`{"created_at":"2026-11-01T10:30:00Z","id":"abc"}`),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeExceptionCursor(token); err == nil {
				t.Errorf("decodeExceptionCursor(%q) succeeded, want an error", token)
			}
		})
	}
}

func TestUnitWriteProblemUsesRFC9457Shape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid limit", "limit must not exceed 200")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if p.Code != "VALIDATION_ERROR" || p.Status != 400 || p.Title != "Invalid limit" {
		t.Errorf("unexpected problem body: %+v", p)
	}
	if p.Type != "https://errors.labbank.example/validation-error" {
		t.Errorf("type = %q, want https://errors.labbank.example/validation-error", p.Type)
	}
}

func TestUnitValidExceptionStatuses(t *testing.T) {
	for _, s := range []string{"OPEN", "UNDER_REVIEW", "RESOLVED", "DISMISSED"} {
		if !validExceptionStatuses[s] {
			t.Errorf("status %q should be accepted", s)
		}
	}
	for _, s := range []string{"", "open", "REJECTED", "CLOSED"} {
		if validExceptionStatuses[s] {
			t.Errorf("status %q should be rejected", s)
		}
	}
}

func TestUnitLoadConfigDefaultsAndOverrides(t *testing.T) {
	for _, k := range []string{"SERVICE_NAME", "LISTEN_ADDR", "DB_HOST"} {
		t.Setenv(k, "")
	}
	cfg := loadConfig()
	if cfg.ServiceName != "S6-RCN" || cfg.ListenAddr != ":8086" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}

	t.Setenv("LISTEN_ADDR", "0.0.0.0:8086")
	t.Setenv("DB_HOST", "postgres")
	cfg = loadConfig()
	if cfg.ListenAddr != "0.0.0.0:8086" || cfg.DBHost != "postgres" {
		t.Errorf("environment overrides were ignored: %+v", cfg)
	}
}

func TestUnitLoadS2ConfigTrimsTrailingSlash(t *testing.T) {
	t.Setenv("NEIGHBOR_BASE_URL", "")
	t.Setenv("S2_LEDGER_BASE_URL", "http://ledger.example:8082/")
	cfg := LoadS2Config()
	if cfg.BaseURL != "http://ledger.example:8082" {
		t.Errorf("BaseURL = %q, want trailing slash removed", cfg.BaseURL)
	}
}
