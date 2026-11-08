package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// ---- API tests that need PostgreSQL (skipped when TEST_DATABASE_URL is not set) ----
// They cover the matrix rows T04, T11, T13, T14 and T30.

func lab5SeededServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	db := lab5IsolatedDB(t) // skips the test when TEST_DATABASE_URL is missing
	srv := &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}
	if err := srv.initDB(); err != nil {
		t.Fatalf("initDB: %v", err)
	}
	return srv, lab5Router(srv)
}

// T04: readiness succeeds when PostgreSQL is up.
func TestAPIReadyReturns200WhenDatabaseIsUp(t *testing.T) {
	_, router := lab5SeededServer(t)

	rec := lab5Do(router, http.MethodGet, "/ready", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body ReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Status != "ready" || body.Database != "connected" {
		t.Errorf("unexpected body: %+v", body)
	}
}

// T11: an empty result must be [] (never null) and next_cursor must be null.
func TestAPIListExceptionsEmptyResultIsEmptyArrayNotNull(t *testing.T) {
	_, router := lab5SeededServer(t)

	// The seed data has no DISMISSED exceptions.
	rec := lab5Do(router, http.MethodGet, "/api/v1/exceptions?status=DISMISSED", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	items, ok := body["items"].([]any)
	if !ok {
		t.Fatalf("items must be a JSON array, got %T (body: %s)", body["items"], rec.Body.String())
	}
	if len(items) != 0 {
		t.Errorf("expected no items, got %d", len(items))
	}
	if body["next_cursor"] != nil {
		t.Errorf("next_cursor = %v, want null", body["next_cursor"])
	}
}

// T13: a valid UUID that does not exist anywhere -> 404 Problem Details.
func TestAPIAccountExceptionsUnknownAccountReturns404(t *testing.T) {
	_, router := lab5SeededServer(t)

	rec := lab5Do(router, http.MethodGet, "/api/v1/accounts/"+uuid.NewString()+"/exceptions", nil)

	lab5AssertProblem(t, rec, http.StatusNotFound, "NOT_FOUND")
}

// T14: the fixed seed account of the neighbour team returns its two exceptions.
func TestAPIAccountExceptionsSeededAccountReturnsItems(t *testing.T) {
	_, router := lab5SeededServer(t)

	rec := lab5Do(router, http.MethodGet, "/api/v1/accounts/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/exceptions", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var items []AccountExceptionItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want the 2 seeded exceptions", len(items))
	}
	types := map[string]bool{}
	for _, it := range items {
		types[it.DiscrepancyType] = true
		if it.ReferenceID == "" || it.Currency != "IRR" {
			t.Errorf("incomplete item: %+v", it)
		}
		if it.Difference != it.ExpectedAmount-it.ActualAmount {
			t.Errorf("difference %d != expected %d - actual %d", it.Difference, it.ExpectedAmount, it.ActualAmount)
		}
	}
	if !types["AMOUNT_MISMATCH"] || !types["MISSING_IN_SETTLEMENT"] {
		t.Errorf("unexpected discrepancy types: %v", types)
	}
}

// Edge case: a known account without exceptions answers 200 with [] (not 404, not null).
func TestAPIAccountExceptionsKnownAccountWithoutExceptionsReturnsEmptyArray(t *testing.T) {
	_, router := lab5SeededServer(t)

	rec := lab5Do(router, http.MethodGet, "/api/v1/accounts/b1ffcd00-0d1c-4fe9-cc7e-7cc0ce491b22/exceptions", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
		t.Errorf("body = %s, want []", got)
	}
}

// T30: triggering a job stores it and returns 201. NOTE: the job logic is still hard-coded
// (350 matched, 0 discrepancies); update this test when the real engine replaces it.
func TestAPITriggerJobCreatesAndStoresJob(t *testing.T) {
	srv, router := lab5SeededServer(t)

	rec := lab5Do(router, http.MethodPost, "/api/v1/reconciliations/jobs", nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var job ReconciliationJob
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, err := uuid.Parse(job.ID); err != nil {
		t.Errorf("job id %q is not a UUID", job.ID)
	}
	if job.Status != "COMPLETED" || !job.ToTime.After(job.FromTime) || job.TotalAmountReconciled.Currency != "IRR" {
		t.Errorf("unexpected job: %+v", job)
	}

	var n int
	if err := srv.db.QueryRow(`SELECT COUNT(*) FROM reconciliation_jobs WHERE id = $1`, job.ID).Scan(&n); err != nil {
		t.Fatalf("query job: %v", err)
	}
	if n != 1 {
		t.Errorf("job %s was not stored in the database", job.ID)
	}
}
