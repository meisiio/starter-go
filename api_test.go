package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---- API tests: real handlers, real middleware, no database needed -------------
//
// The error paths below are answered before any SQL runs, so Server{db: nil} is enough.

// lab5Router mirrors the route table of main() so requests reach the handlers exactly as
// in production (including r.PathValue). Keep it in sync with main().
func lab5Router(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.HealthCheckHandler)
	mux.HandleFunc("GET /ready", srv.ReadinessCheckHandler)
	mux.HandleFunc("GET /api/v1/exceptions", srv.ListExceptionsHandler)
	mux.HandleFunc("GET /api/v1/accounts/{id}/exceptions", srv.GetAccountExceptionsHandler)
	mux.HandleFunc("POST /api/v1/reconciliations/jobs", srv.TriggerJobHandler)
	return srv.correlationMiddleware(corsMiddleware(mux))
}

func lab5Do(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func lab5AssertProblem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) ProblemDetails {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if p.Code != wantCode || p.Status != wantStatus {
		t.Errorf("problem code/status = %s/%d, want %s/%d", p.Code, p.Status, wantCode, wantStatus)
	}
	if p.Type == "" || p.Title == "" || p.Detail == "" {
		t.Errorf("problem is missing type, title or detail: %+v", p)
	}
	return p
}

func lab5NoDBServer() *Server {
	return &Server{cfg: Config{ServiceName: "S6-RCN"}}
}

// Success path: GET /health
func TestAPIHealthReturns200AndMatchesSchema(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body.Status != "ok" || body.Service != "S6-RCN" || body.Version == "" {
		t.Errorf("unexpected body: %+v", body)
	}
	if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
		t.Errorf("timestamp %q is not RFC 3339: %v", body.Timestamp, err)
	}
}

func TestAPIPostHealthReturns405(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodPost, "/health", nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /health status = %d, want 405", rec.Code)
	}
}

// Error paths of GET /api/v1/exceptions
func TestAPIListExceptionsRejectsLimitAbove200(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?limit=201", nil)
	lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
}

func TestAPIListExceptionsRejectsNonPositiveOrNonNumericLimit(t *testing.T) {
	for _, limit := range []string{"0", "-5", "abc"} {
		rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?limit="+limit, nil)
		lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
	}
}

func TestAPIListExceptionsRejectsInvalidCursor(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?cursor=abc", nil)
	lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
}

func TestAPIListExceptionsRejectsUnknownStatus(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?status=CLOSED", nil)
	lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
}

// Error path of the provider endpoint used by the neighbour team (Team 1)
func TestAPIAccountExceptionsRejectsMalformedUUID(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/accounts/not-a-uuid/exceptions", nil)
	lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
}

// Error path: readiness must answer 503 + Problem Details when PostgreSQL is unreachable.
func TestAPIReadyReturns503WhenDatabaseIsDown(t *testing.T) {
	// Port 1 refuses connections immediately; sql.Open itself does not connect.
	db, err := sql.Open("postgres", "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	srv := &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}
	rec := lab5Do(lab5Router(srv), http.MethodGet, "/ready", nil)
	lab5AssertProblem(t, rec, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE")
}

// GEN-08: the correlation id is echoed and also shows up inside Problem Details.
func TestAPICorrelationIDIsEchoedAndAppearsInProblemDetails(t *testing.T) {
	const cid = "lab5-correlation-id-123"
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?limit=999",
		map[string]string{"X-Correlation-ID": cid})

	if got := rec.Header().Get("X-Correlation-ID"); got != cid {
		t.Errorf("X-Correlation-ID header = %q, want %q", got, cid)
	}
	p := lab5AssertProblem(t, rec, http.StatusBadRequest, "VALIDATION_ERROR")
	if p.CorrelationID != cid {
		t.Errorf("problem.correlation_id = %q, want %q", p.CorrelationID, cid)
	}
}

func TestAPICorrelationIDIsGeneratedWhenMissing(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodGet, "/health", nil)
	if rec.Header().Get("X-Correlation-ID") == "" {
		t.Error("the service must create an X-Correlation-ID when the caller sends none")
	}
}
