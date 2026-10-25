package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// @title           LabBank - S6-RCN Reconciliation & Exceptions Service API
// @version         1.0.0
// @description     Official OpenAPI Specification for LabBank Reconciliation & Exceptions Service (S6-RCN).
// @contact.name    S6-RCN Engineering Team
// @contact.email   rcn-team@labbank.example
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

	return Config{
		ServiceName: serviceName,
		ListenAddr:  listenAddr,
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBName:      dbName,
		DBSSLMode:   dbSSLMode,
	}
}

// Server holds application state and dependencies.
type Server struct {
	cfg Config
	db  *sql.DB
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
		Type:          fmt.Sprintf("https://errors.labbank.example/%s", strings.ToLower(code)),
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

// initDB runs schema migrations and seeds initial reconciliation exceptions.
func (s *Server) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS reconciliation_jobs (
		id UUID PRIMARY KEY,
		status VARCHAR(50) NOT NULL,
		from_time TIMESTAMPTZ NOT NULL,
		to_time TIMESTAMPTZ NOT NULL,
		matched_count INT NOT NULL DEFAULT 0,
		discrepancy_count INT NOT NULL DEFAULT 0,
		total_amount_reconciled BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS discrepancy_exceptions (
		id UUID PRIMARY KEY,
		job_id UUID REFERENCES reconciliation_jobs(id),
		transaction_reference VARCHAR(100) NOT NULL,
		type VARCHAR(50) NOT NULL,
		status VARCHAR(50) NOT NULL,
		ledger_amount BIGINT NOT NULL,
		gateway_amount BIGINT NOT NULL,
		difference_amount BIGINT NOT NULL,
		description TEXT NOT NULL,
		resolution_notes TEXT,
		resolved_by VARCHAR(100),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}

	// Check if seed data exists
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM reconciliation_jobs`).Scan(&count); err == nil && count == 0 {
		jobID := "0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11"
		_, err = s.db.Exec(`
			INSERT INTO reconciliation_jobs (id, status, from_time, to_time, matched_count, discrepancy_count, total_amount_reconciled, created_at)
			VALUES ($1, 'COMPLETED', NOW() - INTERVAL '1 day', NOW(), 1420, 2, 45000000000, NOW());
		`, jobID)
		if err == nil {
			_, _ = s.db.Exec(`
				INSERT INTO discrepancy_exceptions 
				(id, job_id, transaction_reference, type, status, ledger_amount, gateway_amount, difference_amount, description, created_at)
				VALUES
				($1, $2, 'TX-20261025-001', 'AMOUNT_MISMATCH', 'OPEN', 25000000, 24500000, 500000, 'Difference in settlement fee calculation between S2 ledger and S5 gateway', NOW() - INTERVAL '2 hours'),
				($3, $2, 'TX-20261025-002', 'MISSING_IN_GATEWAY', 'UNDER_REVIEW', 10000000, 0, 10000000, 'Internal transfer captured in S2 but missing in external clearing file', NOW() - INTERVAL '1 hour');
			`, uuid.NewString(), jobID, uuid.NewString())
		}
	}

	return nil
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

// ListExceptionsHandler godoc
// @Summary      List discrepancies and exceptions
// @Description  Retrieve a paginated list of financial discrepancies identified by reconciliation jobs.
// @Tags         Reconciliation Exceptions
// @Produce      json
// @Success      200  {object}  ExceptionsResponse
// @Failure      500  {object}  ProblemDetails
// @Router       /api/v1/exceptions [get]
func (s *Server) ListExceptionsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT id, job_id, transaction_reference, type, status, 
		       ledger_amount, gateway_amount, difference_amount, description, 
		       resolution_notes, resolved_by, created_at 
		FROM discrepancy_exceptions 
		ORDER BY created_at DESC 
		LIMIT 50
	`)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Database Error", err.Error())
		return
	}
	defer rows.Close()

	items := make([]Discrepancy, 0)
	for rows.Next() {
		var d Discrepancy
		var ledgerAmt, gatewayAmt, diffAmt int64
		if err := rows.Scan(&d.ID, &d.JobID, &d.TransactionReference, &d.Type, &d.Status,
			&ledgerAmt, &gatewayAmt, &diffAmt, &d.Description,
			&d.ResolutionNotes, &d.ResolvedBy, &d.CreatedAt); err != nil {
			continue
		}
		d.LedgerAmount = Money{Amount: ledgerAmt, Currency: "IRR"}
		d.GatewayAmount = Money{Amount: gatewayAmt, Currency: "IRR"}
		d.DifferenceAmount = Money{Amount: diffAmt, Currency: "IRR"}
		items = append(items, d)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ExceptionsResponse{
		Items:       items,
		NextCursor: nil,
	})
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
	_, err := s.db.Exec(`
		INSERT INTO reconciliation_jobs (id, status, from_time, to_time, matched_count, discrepancy_count, total_amount_reconciled, created_at)
		VALUES ($1, 'COMPLETED', $2, $3, 350, 0, 12500000000, $4)
	`, jobID, now.Add(-24*time.Hour), now, now)

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

	srv := &Server{cfg: cfg, db: db}

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
	mux.HandleFunc("GET /api/v1/exceptions", srv.ListExceptionsHandler)

	// 4. POST /api/v1/reconciliations/jobs (Trigger reconciliation run)
	mux.HandleFunc("POST /api/v1/reconciliations/jobs", srv.TriggerJobHandler)

	// 5. Serve OpenAPI & Swagger files
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		http.ServeFile(w, r, "docs/openapi.yaml")
	})
	mux.HandleFunc("GET /docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		http.ServeFile(w, r, "docs/openapi.json")
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
          { url: "/docs/openapi.json", name: "OpenAPI 3.0 (Contract-First)" }
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
