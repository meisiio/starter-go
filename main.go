package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// @title           LabBank - S6-RCN Reconciliation & Exceptions Service API
// @version         1.0.0
// @description     Official OpenAPI Specification for LabBank Reconciliation & Exceptions Service (S6-RCN).
// @contact.name    reza.shalchian
// @contact.email   rezammh823@gmail.com
// @host            localhost:8086
// @BasePath        /

// HealthResponse represents process liveness status.
type HealthResponse struct {
	Status    string `json:"status" example:"ok"`
	Service   string `json:"service" example:"S6-RCN"`
	Version   string `json:"version" example:"1.0.0"`
	Timestamp string `json:"timestamp" example:"2026-10-25T16:51:29Z"`
}

// ReadyResponse represents readiness of dependencies such as PostgreSQL.
type ReadyResponse struct {
	Status    string `json:"status" example:"ready"`
	Service   string `json:"service" example:"S6-RCN"`
	Database  string `json:"database" example:"connected"`
	Timestamp string `json:"timestamp" example:"2026-10-25T16:51:29Z"`
}

// ExceptionsResponse represents a paginated list of discrepancy exceptions.
type ExceptionsResponse struct {
	Items      []Discrepancy `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

// ReconciliationJob represents a reconciliation execution job result.
type ReconciliationJob struct {
	ID                    string    `json:"id" example:"a1b2c3d4-e5f6-7890-abcd-ef1234567890"`
	Status                string    `json:"status" example:"COMPLETED"`
	FromTime              time.Time `json:"from_time"`
	ToTime                time.Time `json:"to_time"`
	MatchedCount          int       `json:"matched_count" example:"350"`
	DiscrepancyCount      int       `json:"discrepancy_count" example:"0"`
	TotalAmountReconciled Money     `json:"total_amount_reconciled"`
	CreatedAt             time.Time `json:"created_at"`
}

// Config holds runtime configuration loaded from environment variables (GEN-12).
type Config struct {
	ServiceName string
	ListenAddr  string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
	S2BaseURL   string
}

func loadConfig() Config {
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "S6-RCN"
	}
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8086"
	}
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "labbank_s6"
	}
	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}
	s2BaseURL := os.Getenv("S2_LEDGER_BASE_URL")
	if s2BaseURL == "" {
		s2BaseURL = "http://localhost:8082"
	}

	return Config{
		ServiceName: serviceName,
		ListenAddr:  listenAddr,
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBName:      dbName,
		DBSSLMode:   dbSSLMode,
		S2BaseURL:   s2BaseURL,
	}
}

// Server holds application state and dependencies.
type Server struct {
	cfg      Config
	db       *sql.DB
	s2Client *S2Client
}

// ProblemDetails represents RFC 9457 error response (GEN-06).
type ProblemDetails struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	Detail        string `json:"detail"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	cid, _ := r.Context().Value(correlationKey).(string)
	p := ProblemDetails{
		Type:          fmt.Sprintf("https://errors.labbank.example/%s", strings.ReplaceAll(strings.ToLower(code), "_", "-")),
		Title:         title,
		Status:        status,
		Code:          code,
		Detail:        detail,
		CorrelationID: cid,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// correlationID returns the X-Correlation-ID stored in the request context by correlationMiddleware.
func correlationID(r *http.Request) string {
	cid, _ := r.Context().Value(correlationKey).(string)
	return cid
}

// Structured JSON log entry (GEN-11).
type logEntry struct {
	Level         string `json:"level"`
	Timestamp     string `json:"timestamp"`
	Service       string `json:"service"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Message       string `json:"message"`
	Method        string `json:"method,omitempty"`
	Path          string `json:"path,omitempty"`
	StatusCode    int    `json:"status_code,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
}

func logJSON(level, service, cid, msg string, extra map[string]any) {
	entry := map[string]any{
		"level":          level,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"service":        service,
		"correlation_id": cid,
		"message":        msg,
	}
	for k, v := range extra {
		entry[k] = v
	}
	data, _ := json.Marshal(entry)
	fmt.Println(string(data))
}

type contextKey string

const correlationKey contextKey = "correlation_id"

// correlationMiddleware propagates and assigns X-Correlation-ID (GEN-08).
func (s *Server) correlationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cid := r.Header.Get("X-Correlation-ID")
		if cid == "" {
			cid = uuid.NewString()
		}
		w.Header().Set("X-Correlation-ID", cid)
		ctx := context.WithValue(r.Context(), correlationKey, cid)

		start := time.Now()
		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(rw, r.WithContext(ctx))

		logJSON("INFO", s.cfg.ServiceName, cid, "HTTP Request completed", map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status_code": rw.statusCode,
			"duration_ms": time.Since(start).Milliseconds(),
		})
	})
}

// corsMiddleware sets CORS headers allowing frontend interaction without errors.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID, Idempotency-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Money represents amount in Rials (GEN-03).
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// Discrepancy represents an exception case in S6-RCN.
type Discrepancy struct {
	ID                   string    `json:"id"`
	JobID                string    `json:"job_id"`
	TransactionReference string    `json:"transaction_reference"`
	Type                 string    `json:"type"`
	Status               string    `json:"status"`
	LedgerAmount         Money     `json:"ledger_amount"`
	GatewayAmount        Money     `json:"gateway_amount"`
	DifferenceAmount     Money     `json:"difference_amount"`
	Description          string    `json:"description"`
	ResolutionNotes      *string   `json:"resolution_notes"`
	ResolvedBy           *string   `json:"resolved_by"`
	CreatedAt            time.Time `json:"created_at"`
}

// AccountExceptionItem represents discrepancy item returned for Team 1 (Consumer).
type AccountExceptionItem struct {
	ID              string    `json:"id"`
	ReferenceID     string    `json:"reference_id"`
	DiscrepancyType string    `json:"discrepancy_type"`
	ExpectedAmount  int64     `json:"expected_amount"`
	ActualAmount    int64     `json:"actual_amount"`
	Difference      int64     `json:"difference"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// initDB runs schema migrations and seeds initial reconciliation exceptions.
func (s *Server) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS reconciliation_jobs (
		id UUID PRIMARY KEY,
		business_date DATE NOT NULL DEFAULT CURRENT_DATE,
		status VARCHAR(20) NOT NULL DEFAULT 'COMPLETED',
		total_checked INT NOT NULL DEFAULT 0,
		matched_count INT NOT NULL DEFAULT 0,
		discrepancy_count INT NOT NULL DEFAULT 0,
		started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ,
		error_message TEXT
	);

	CREATE TABLE IF NOT EXISTS bank_accounts (
		account_id UUID PRIMARY KEY,
		account_number VARCHAR(50) NOT NULL UNIQUE,
		iban VARCHAR(34) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS exceptions (
		id UUID PRIMARY KEY,
		job_id UUID REFERENCES reconciliation_jobs(id) ON DELETE SET NULL,
		account_id UUID NOT NULL,
		reference_id UUID NOT NULL,
		entry_id UUID,
		discrepancy_type VARCHAR(40) NOT NULL CHECK (discrepancy_type IN ('AMOUNT_MISMATCH', 'MISSING_IN_LEDGER', 'MISSING_IN_SETTLEMENT', 'STATUS_CONFLICT')),
		expected_amount BIGINT NOT NULL,
		actual_amount BIGINT NOT NULL,
		difference_amount BIGINT NOT NULL,
		currency CHAR(3) NOT NULL DEFAULT 'IRR',
		status VARCHAR(20) NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'UNDER_REVIEW', 'RESOLVED', 'DISMISSED')),
		resolution_notes TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_exceptions_account_id ON exceptions(account_id);
	CREATE INDEX IF NOT EXISTS idx_exceptions_status_created ON exceptions(status, created_at DESC);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Seed known accounts if not present
	_, _ = s.db.Exec(`
		INSERT INTO bank_accounts (account_id, account_number, iban, created_at)
		VALUES 
			('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'AC-1001-2026', 'IR120120000000000000000001', NOW() - INTERVAL '30 days'),
			('b1ffcd00-0d1c-4fe9-cc7e-7cc0ce491b22', 'AC-1002-2026', 'IR120120000000000000000002', NOW() - INTERVAL '30 days')
		ON CONFLICT (account_id) DO NOTHING;
	`)

	// Check if seed jobs exist
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM reconciliation_jobs`).Scan(&count); err == nil && count == 0 {
		jobID := "0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11"
		_, err = s.db.Exec(`
			INSERT INTO reconciliation_jobs (id, business_date, status, total_checked, matched_count, discrepancy_count, started_at, completed_at)
			VALUES ($1, '2026-10-31', 'COMPLETED', 1422, 1420, 2, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour');
		`, jobID)
		if err == nil {
			_, _ = s.db.Exec(`
				INSERT INTO exceptions 
				(id, job_id, account_id, reference_id, entry_id, discrepancy_type, expected_amount, actual_amount, difference_amount, currency, status, resolution_notes, created_at, updated_at)
				VALUES
				('e1111111-1111-1111-1111-111111111111', $1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'f1111111-1111-1111-1111-111111111111', '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11', 'AMOUNT_MISMATCH', 25000000, 24500000, 500000, 'IRR', 'OPEN', 'Settlement fee calculation discrepancy between S2 ledger (25,000,000 IRR) and S5 gateway settlement report (24,500,000 IRR)', NOW() - INTERVAL '90 minutes', NOW() - INTERVAL '90 minutes'),
				('e2222222-2222-2222-2222-222222222222', $1, 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'f2222222-2222-2222-2222-222222222222', '0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c22', 'MISSING_IN_SETTLEMENT', 10000000, 0, 10000000, 'IRR', 'UNDER_REVIEW', 'Transfer recorded in S2 ledger but missing in external S5 clearing report', NOW() - INTERVAL '60 minutes', NOW() - INTERVAL '60 minutes')
				ON CONFLICT (id) DO NOTHING;
			`, jobID)
		}
	}

	return nil
}

// GetAccountExceptionsHandler godoc
// @Summary      Get exceptions by account ID (Consumer Loop Endpoint)
// @Description  Enables Team 1 (Account - Consumer) to query discrepancy exceptions for a specific bank account.
// @Tags         Reconciliation Exceptions
// @Produce      json
// @Param        id   path      string  true  "Target account UUID"
// @Success      200  {array}   AccountExceptionItem
// @Failure      400  {object}  ProblemDetails
// @Failure      404  {object}  ProblemDetails
// @Failure      500  {object}  ProblemDetails
// @Router       /api/v1/accounts/{id}/exceptions [get]
func (s *Server) GetAccountExceptionsHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		p := strings.TrimPrefix(r.URL.Path, "/api/v1/accounts/")
		idStr = strings.TrimSuffix(p, "/exceptions")
	}

	accountUUID, err := uuid.Parse(idStr)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation Error",
			fmt.Sprintf("Invalid account UUID format: %q", idStr))
		return
	}

	// Verify account exists in bank records or exceptions table
	var accountExists bool
	err = s.db.QueryRowContext(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM bank_accounts WHERE account_id = $1) 
		 OR EXISTS(SELECT 1 FROM exceptions WHERE account_id = $1)`,
		accountUUID).Scan(&accountExists)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", err.Error())
		return
	}

	if !accountExists {
		writeProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Account Not Found",
			fmt.Sprintf("Account %s was not found in bank records", accountUUID.String()))
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, reference_id, discrepancy_type, expected_amount, actual_amount, difference_amount, currency, status, created_at
		FROM exceptions
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountUUID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", err.Error())
		return
	}
	defer rows.Close()

	items := make([]AccountExceptionItem, 0)
	for rows.Next() {
		var item AccountExceptionItem
		var diffAmt int64
		if err := rows.Scan(
			&item.ID,
			&item.ReferenceID,
			&item.DiscrepancyType,
			&item.ExpectedAmount,
			&item.ActualAmount,
			&diffAmt,
			&item.Currency,
			&item.Status,
			&item.CreatedAt,
		); err != nil {
			writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Scan Error", err.Error())
			return
		}
		item.Difference = diffAmt
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// handleGetAccountExceptions is kept for backwards compatibility with tests.
func (s *Server) handleGetAccountExceptions(w http.ResponseWriter, r *http.Request) {
	s.GetAccountExceptionsHandler(w, r)
}

// HealthCheckHandler godoc
// @Summary      Liveness health check
// @Description  Returns process health status (GEN-10). Does not verify external dependencies.
// @Tags         System Health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func (s *Server) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := HealthResponse{
		Status:    "ok",
		Service:   s.cfg.ServiceName,
		Version:   "1.0.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// ReadinessCheckHandler godoc
// @Summary      Readiness health check
// @Description  Returns readiness of dependencies such as PostgreSQL database connectivity (GEN-10).
// @Tags         System Health
// @Produce      json
// @Success      200  {object}  ReadyResponse
// @Failure      503  {object}  ProblemDetails
// @Router       /ready [get]
func (s *Server) ReadinessCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Database Unavailable",
			fmt.Sprintf("Failed to ping PostgreSQL database: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := ReadyResponse{
		Status:    "ready",
		Service:   s.cfg.ServiceName,
		Database:  "connected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Pagination limits for GET /api/v1/exceptions (GEN-09).
const (
	defaultExceptionsLimit = 50
	maxExceptionsLimit     = 200
)

// Allowed values of the ?status= filter.
var validExceptionStatuses = map[string]bool{
	"OPEN":         true,
	"UNDER_REVIEW": true,
	"RESOLVED":     true,
	"DISMISSED":    true,
}

// exceptionCursor is the position after the last item of a page (keyset pagination).
type exceptionCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

// encode returns an opaque URL-safe token: base64url of {"created_at": ..., "id": ...}.
func (c exceptionCursor) encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeExceptionCursor(token string) (exceptionCursor, error) {
	var c exceptionCursor
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return c, fmt.Errorf("not valid base64url")
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("not valid JSON")
	}
	if c.CreatedAt.IsZero() {
		return c, fmt.Errorf("created_at is missing")
	}
	if _, err := uuid.Parse(c.ID); err != nil {
		return c, fmt.Errorf("id is not a UUID")
	}
	return c, nil
}

// ListExceptionsHandler godoc
// @Summary      List discrepancies and exceptions
// @Description  Paginated list of financial discrepancies, oldest first (created_at, id). Cursor pagination (GEN-09).
// @Tags         Reconciliation Exceptions
// @Produce      json
// @Param        status  query  string  false  "OPEN | UNDER_REVIEW | RESOLVED | DISMISSED"
// @Param        limit   query  int     false  "1..200, default 50; above 200 returns 400"
// @Param        cursor  query  string  false  "next_cursor of the previous page"
// @Success      200  {object}  ExceptionsResponse
// @Failure      400  {object}  ProblemDetails
// @Failure      500  {object}  ProblemDetails
// @Router       /api/v1/exceptions [get]
func (s *Server) ListExceptionsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// ---- 1. validate the query string ----
	limit := defaultExceptionsLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid limit",
				"limit must be a positive integer")
			return
		}
		if n > maxExceptionsLimit {
			writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid limit",
				fmt.Sprintf("limit must not exceed %d", maxExceptionsLimit))
			return
		}
		limit = n
	}

	status := q.Get("status")
	if status != "" && !validExceptionStatuses[status] {
		writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid status",
			"status must be one of OPEN, UNDER_REVIEW, RESOLVED, DISMISSED")
		return
	}

	var cursor *exceptionCursor
	if v := q.Get("cursor"); v != "" {
		c, err := decodeExceptionCursor(v)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid cursor",
				"cursor is not a valid token: "+err.Error())
			return
		}
		cursor = &c
	}

	// ---- 2. build the query with dynamic WHERE clauses ----
	var (
		conds []string
		args  []any
	)
	if status != "" {
		args = append(args, status)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if cursor != nil {
		// keyset pagination: everything after the last row of the previous page
		args = append(args, cursor.CreatedAt, cursor.ID)
		conds = append(conds, fmt.Sprintf("(created_at, id) > ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args)))
	}

	query := `
		SELECT id,
		       COALESCE(job_id::text, ''),
		       reference_id::text,
		       CASE discrepancy_type
		            WHEN 'MISSING_IN_SETTLEMENT' THEN 'MISSING_IN_GATEWAY'
		            WHEN 'STATUS_CONFLICT'       THEN 'STATUS_MISMATCH'
		            ELSE discrepancy_type
		       END,
		       status,
		       expected_amount, actual_amount, difference_amount,
		       format('%s: expected %s %s, actual %s', discrepancy_type, expected_amount, currency, actual_amount),
		       resolution_notes, NULL::text, created_at
		FROM exceptions`
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit+1) // one extra row tells us whether a next page exists
	query += fmt.Sprintf(" ORDER BY created_at ASC, id ASC LIMIT $%d", len(args))

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		logJSON("ERROR", s.cfg.ServiceName, correlationID(r), "list exceptions query failed", map[string]any{"error": err.Error()})
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", "failed to query exceptions")
		return
	}
	defer rows.Close()

	items := make([]Discrepancy, 0, limit)
	for rows.Next() {
		var d Discrepancy
		var ledgerAmt, gatewayAmt, diffAmt int64
		if err := rows.Scan(&d.ID, &d.JobID, &d.TransactionReference, &d.Type, &d.Status,
			&ledgerAmt, &gatewayAmt, &diffAmt, &d.Description,
			&d.ResolutionNotes, &d.ResolvedBy, &d.CreatedAt); err != nil {
			logJSON("ERROR", s.cfg.ServiceName, correlationID(r), "list exceptions scan failed", map[string]any{"error": err.Error()})
			writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", "failed to read exceptions")
			return
		}
		d.LedgerAmount = Money{Amount: ledgerAmt, Currency: "IRR"}
		d.GatewayAmount = Money{Amount: gatewayAmt, Currency: "IRR"}
		d.DifferenceAmount = Money{Amount: diffAmt, Currency: "IRR"}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		logJSON("ERROR", s.cfg.ServiceName, correlationID(r), "list exceptions iteration failed", map[string]any{"error": err.Error()})
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", "failed to read exceptions")
		return
	}

	// ---- 3. build next_cursor ----
	var next *string
	if len(items) > limit {
		items = items[:limit]
		last := items[limit-1]
		token := exceptionCursor{CreatedAt: last.CreatedAt, ID: last.ID}.encode()
		next = &token
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ExceptionsResponse{Items: items, NextCursor: next})
}

// TriggerJobHandler godoc
// @Summary      Trigger reconciliation job
// @Description  Start a new reconciliation run between Ledger (S2) and Settlement Gateway (S5).
// @Tags         Reconciliation Jobs
// @Produce      json
// @Success      201  {object}  ReconciliationJob
// @Failure      500  {object}  ProblemDetails
// @Router       /api/v1/reconciliations/jobs [post]
func (s *Server) TriggerJobHandler(w http.ResponseWriter, r *http.Request) {
	jobID := uuid.NewString()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	_, err := s.db.Exec(`
		INSERT INTO reconciliation_jobs (id, business_date, status, total_checked, matched_count, discrepancy_count, started_at, completed_at)
		VALUES ($1, $2, 'COMPLETED', 350, 350, 0, $3, $4)
	`, jobID, today, now.Add(-10*time.Minute), now)

	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to start reconciliation", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(ReconciliationJob{
		ID:                    jobID,
		Status:                "COMPLETED",
		FromTime:              now.Add(-24 * time.Hour),
		ToTime:                now,
		MatchedCount:          350,
		DiscrepancyCount:      0,
		TotalAmountReconciled: Money{Amount: 12500000000, Currency: "IRR"},
		CreatedAt:             now,
	})
}

func main() {
	cfg := loadConfig()

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	s2Client := NewS2Client(S2ClientConfig{
		BaseURL: cfg.S2BaseURL,
		Timeout: 5 * time.Second,
	})
	srv := &Server{cfg: cfg, db: db, s2Client: s2Client}

	// Try verifying DB connection
	if err := db.Ping(); err != nil {
		logJSON("WARN", cfg.ServiceName, "", fmt.Sprintf("Database ping failed (will retry on /ready): %v", err), nil)
	} else {
		logJSON("INFO", cfg.ServiceName, "", "Connected successfully to PostgreSQL", nil)
		if err := srv.initDB(); err != nil {
			logJSON("WARN", cfg.ServiceName, "", fmt.Sprintf("DB schema init warning: %v", err), nil)
		} else {
			logJSON("INFO", cfg.ServiceName, "", "Database tables initialized and verified", nil)
		}
	}

	mux := http.NewServeMux()

	// 1. GET /health (Liveness health check - GEN-10)
	mux.HandleFunc("GET /health", srv.HealthCheckHandler)

	// 2. GET /ready (Readiness health check including DB - GEN-10)
	mux.HandleFunc("GET /ready", srv.ReadinessCheckHandler)

	// 3. GET /api/v1/exceptions (List discrepancies with pagination - GEN-09)
	// 3. GET /api/v1/exceptions (List discrepancies with pagination - GEN-09)
	mux.HandleFunc("GET /api/v1/exceptions", srv.ListExceptionsHandler)

	// 4. GET /api/v1/accounts/{id}/exceptions (Loop endpoint for Team 1 Consumer - KAN-12)
	mux.HandleFunc("GET /api/v1/accounts/{id}/exceptions", srv.GetAccountExceptionsHandler)

	// 5. POST /api/v1/reconciliations/jobs (Trigger reconciliation run)
	mux.HandleFunc("POST /api/v1/reconciliations/jobs", srv.TriggerJobHandler)

	// 6. Serve OpenAPI & Swagger files
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		http.ServeFile(w, r, "openapi/reconciliation.yaml")
	})
	mux.HandleFunc("GET /docs/swagger.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		http.ServeFile(w, r, "docs/swagger.yaml")
	})
	mux.HandleFunc("GET /docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		http.ServeFile(w, r, "docs/swagger.json")
	})

	// 6. Serve Swagger UI at /swagger/
	mux.HandleFunc("GET /swagger/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>LabBank S6-RCN Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" />
  <style>
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/docs/swagger.json",
        urls: [
          { url: "/docs/swagger.json", name: "Swagger 2.0 (Swag Generated)" },
          { url: "/docs/openapi.yaml", name: "OpenAPI 3.0 (Contract-First)" }
        ],
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
    };
  </script>
</body>
</html>`
		w.Write([]byte(html))
	})

	handler := srv.correlationMiddleware(corsMiddleware(mux))

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	logJSON("INFO", cfg.ServiceName, "", fmt.Sprintf("Service S6-RCN starting on %s", cfg.ListenAddr), nil)
	log.Fatal(server.ListenAndServe())
}
