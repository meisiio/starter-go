package main

import (
	"encoding/json"
	"net/http"
)

// RFC 9457 error body
type Problem struct {
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	Detail        string `json:"detail,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func writeProblem(w http.ResponseWriter, status int, code, title, detail, correlationID string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Title:         title,
		Status:        status,
		Code:          code,
		Detail:        detail,
		CorrelationID: correlationID,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
