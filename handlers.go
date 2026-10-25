package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// handleReady currently only checks that a DB URL is configured.
// TODO: replace with a real Postgres ping.
func handleReady(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("DATABASE_URL") == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"reason": "database not configured",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type createRunRequest struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

type runResponse struct {
	ID          string    `json:"id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Status      string    `json:"status"`
}

// handleCreateRun is a stub: it validates input and returns a fake run.
// TODO once a store exists:
//   - persist the run
//   - honor Idempotency-Key (same key + same body -> same response;
//     same key + different body -> 409 IDEMPOTENCY_KEY_REUSED)
//   - reject overlapping in-progress runs with 409 RECONCILIATION_RUN_IN_PROGRESS
func handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "ERROR_VALIDATION", "Invalid request body", err.Error(), correlationID(r))
		return
	}
	if req.PeriodEnd.Before(req.PeriodStart) {
		writeProblem(w, http.StatusBadRequest, "ERROR_VALIDATION", "period_end must not be before period_start", "", correlationID(r))
		return
	}

	writeJSON(w, http.StatusCreated, runResponse{
		ID:          newUUID(),
		PeriodStart: req.PeriodStart,
		PeriodEnd:   req.PeriodEnd,
		Status:      "pending",
	})
}

type listCasesResponse struct {
	Items      []map[string]string `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

// handleListCases is a stub returning an empty page.
// TODO: back with a store, support ?status= filter, limit/cursor paging.
func handleListCases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, listCasesResponse{Items: []map[string]string{}, NextCursor: nil})
}
