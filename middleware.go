package main

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"net/http"
	"time"
)

type ctxKey string

const correlationIDKey ctxKey = "correlation_id"

// generate correlation-id if missing / use for logging
func withCorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-ID")
		if id == "" {
			id = newUUID()
		}
		w.Header().Set("X-Correlation-ID", id)
		ctx := context.WithValue(r.Context(), correlationIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extracts correlation-id
func correlationID(r *http.Request) string {
	if v, ok := r.Context().Value(correlationIDKey).(string); ok {
		return v
	}
	return ""
}

// simple logging on chain **must run after withCorrelationID**
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logEvent("info", "request", map[string]any{
			"method":         r.Method,
			"path":           r.URL.Path,
			"correlation_id": correlationID(r),
			"duration_ms":    time.Since(start).Milliseconds(),
		})
	})
}

// simple UUIDv4 generator / used as placeholder
func newUUID() string {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
