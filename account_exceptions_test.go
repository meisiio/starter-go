package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccountExceptions_InvalidUUID(t *testing.T) {
	srv := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/accounts/{id}/exceptions", srv.handleGetAccountExceptions)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/invalid-uuid-string/exceptions", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var prob ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to unmarshal ProblemDetails: %v", err)
	}

	if prob.Code != "VALIDATION_ERROR" {
		t.Errorf("expected code VALIDATION_ERROR, got %s", prob.Code)
	}
	if prob.Status != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", prob.Status)
	}
}
