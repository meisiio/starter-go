package main

import (
	"database/sql"
	"net/http"
	"testing"
)

// A closed database makes every query fail, so we can test the 500 paths without a real DB.
func lab5ClosedDBServer(t *testing.T) *Server {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_ = db.Close()
	return &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}
}

func TestAPIListExceptionsReturns500WhenDatabaseFails(t *testing.T) {
	rec := lab5Do(lab5Router(lab5ClosedDBServer(t)), http.MethodGet, "/api/v1/exceptions", nil)
	lab5AssertProblem(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
}

func TestAPIAccountExceptionsReturns500WhenDatabaseFails(t *testing.T) {
	rec := lab5Do(lab5Router(lab5ClosedDBServer(t)), http.MethodGet,
		"/api/v1/accounts/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/exceptions", nil)
	lab5AssertProblem(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
}

func TestAPITriggerJobReturns500WhenDatabaseFails(t *testing.T) {
	rec := lab5Do(lab5Router(lab5ClosedDBServer(t)), http.MethodPost, "/api/v1/reconciliations/jobs", nil)
	lab5AssertProblem(t, rec, http.StatusInternalServerError, "INTERNAL_ERROR")
}

func TestAPICORSPreflightReturns204(t *testing.T) {
	rec := lab5Do(lab5Router(lab5NoDBServer()), http.MethodOptions, "/api/v1/exceptions",
		map[string]string{"Origin": "http://localhost:5173"})
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing CORS header: %v", rec.Header())
	}
}

func TestUnitLoadConfigReadsEveryEnvironmentVariable(t *testing.T) {
	t.Setenv("SERVICE_NAME", "S6-TEST")
	t.Setenv("LISTEN_ADDR", "0.0.0.0:9999")
	t.Setenv("DB_HOST", "db.example")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "u")
	t.Setenv("DB_PASSWORD", "p")
	t.Setenv("DB_NAME", "n")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("S2_LEDGER_BASE_URL", "http://s2.example:1")

	want := Config{ServiceName: "S6-TEST", ListenAddr: "0.0.0.0:9999", DBHost: "db.example", DBPort: "5433",
		DBUser: "u", DBPassword: "p", DBName: "n", DBSSLMode: "require", S2BaseURL: "http://s2.example:1"}
	if got := loadConfig(); got != want {
		t.Errorf("loadConfig() = %+v, want %+v", got, want)
	}
}
