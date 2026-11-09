package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ---- Integration tests: real PostgreSQL ------------------------------------------
//
// They run only when TEST_DATABASE_URL is set (URL form), otherwise they are skipped:
//
//	docker compose up -d postgres
//	export TEST_DATABASE_URL="postgres://admin:password@127.0.0.1:5432/labbank_reconciliation?sslmode=disable"
//	go test -run Integration -v ./...
//
// Every test works in its own throw-away schema, so your normal data is never touched.

func lab5IsolatedDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set: skipping PostgreSQL integration test")
	}

	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	if err := admin.Ping(); err != nil {
		admin.Close()
		t.Fatalf("PostgreSQL is not reachable with TEST_DATABASE_URL: %v", err)
	}

	schema := "lab5_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		admin.Close()
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL must be a postgres:// URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatalf("open test connection: %v", err)
	}
	t.Cleanup(func() { db.Close() }) // runs before the schema is dropped (LIFO)
	return db
}

func lab5CountRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func lab5InsertException(db *sql.DB, status string, createdAt time.Time) error {
	_, err := db.Exec(`
		INSERT INTO exceptions
			(id, account_id, reference_id, discrepancy_type, expected_amount, actual_amount,
			 difference_amount, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'AMOUNT_MISMATCH', 100, 90, 10, 'IRR', $4, $5, $5)`,
		uuid.NewString(), uuid.NewString(), uuid.NewString(), status, createdAt)
	return err
}

// Integration 1: schema creation + seed, and running it twice changes nothing.
func TestIntegrationInitDBCreatesSchemaAndSeedsIdempotently(t *testing.T) {
	db := lab5IsolatedDB(t)
	srv := &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}

	if err := srv.initDB(); err != nil {
		t.Fatalf("first initDB: %v", err)
	}
	exceptions := lab5CountRows(t, db, "exceptions")
	accounts := lab5CountRows(t, db, "bank_accounts")
	jobs := lab5CountRows(t, db, "reconciliation_jobs")
	if exceptions == 0 || accounts == 0 || jobs == 0 {
		t.Fatalf("seed data missing: exceptions=%d accounts=%d jobs=%d", exceptions, accounts, jobs)
	}

	if err := srv.initDB(); err != nil {
		t.Fatalf("second initDB: %v", err)
	}
	if got := lab5CountRows(t, db, "exceptions"); got != exceptions {
		t.Errorf("exceptions changed after second initDB: %d -> %d", exceptions, got)
	}
	if got := lab5CountRows(t, db, "bank_accounts"); got != accounts {
		t.Errorf("bank_accounts changed after second initDB: %d -> %d", accounts, got)
	}
	if got := lab5CountRows(t, db, "reconciliation_jobs"); got != jobs {
		t.Errorf("reconciliation_jobs changed after second initDB: %d -> %d", jobs, got)
	}
}

// Integration 2: the database itself enforces the status vocabulary (DISMISSED allowed, CLOSED not).
func TestIntegrationStatusCheckConstraint(t *testing.T) {
	db := lab5IsolatedDB(t)
	srv := &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}
	if err := srv.initDB(); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	if err := lab5InsertException(db, "DISMISSED", time.Now().UTC()); err != nil {
		t.Errorf("DISMISSED must be accepted: %v", err)
	}
	if err := lab5InsertException(db, "CLOSED", time.Now().UTC()); err == nil {
		t.Error("CLOSED must be rejected by the CHECK constraint")
	}
}

// Integration 3: GET /api/v1/exceptions end to end - cursor pagination over every row, and the status filter.
func TestIntegrationListExceptionsPaginatesAndFilters(t *testing.T) {
	db := lab5IsolatedDB(t)
	srv := &Server{cfg: Config{ServiceName: "S6-RCN"}, db: db}
	if err := srv.initDB(); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	base := time.Now().UTC().Add(time.Hour)
	for i, status := range []string{"OPEN", "DISMISSED", "OPEN"} {
		if err := lab5InsertException(db, status, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("insert %s: %v", status, err)
		}
	}
	total := lab5CountRows(t, db, "exceptions")
	router := lab5Router(srv)

	// Walk every page with limit=2.
	seen := map[string]bool{}
	var last time.Time
	target := "/api/v1/exceptions?limit=2"
	for page := 0; page < 10; page++ {
		rec := lab5Do(router, http.MethodGet, target, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d: status = %d, body: %s", page, rec.Code, rec.Body.String())
		}
		var resp ExceptionsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("page %d: invalid JSON: %v", page, err)
		}
		if len(resp.Items) == 0 || len(resp.Items) > 2 {
			t.Fatalf("page %d: got %d items, want 1..2", page, len(resp.Items))
		}
		for _, it := range resp.Items {
			if seen[it.ID] {
				t.Errorf("exception %s appeared on two pages", it.ID)
			}
			seen[it.ID] = true
			if it.CreatedAt.Before(last) {
				t.Errorf("items are not in chronological order at %s", it.ID)
			}
			last = it.CreatedAt
		}
		if resp.NextCursor == nil {
			break
		}
		target = "/api/v1/exceptions?limit=2&cursor=" + url.QueryEscape(*resp.NextCursor)
	}
	if len(seen) != total {
		t.Errorf("pagination returned %d distinct rows, database has %d", len(seen), total)
	}

	// Status filter.
	rec := lab5Do(router, http.MethodGet, "/api/v1/exceptions?status=DISMISSED", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("filter: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	var filtered ExceptionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("filter: invalid JSON: %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Status != "DISMISSED" {
		t.Errorf("status filter returned %+v, want exactly one DISMISSED item", filtered.Items)
	}
}
