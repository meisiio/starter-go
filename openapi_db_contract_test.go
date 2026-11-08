//go:build openapi

package main

import (
	"net/http"
	"testing"
)

// Success responses (these need PostgreSQL) checked against openapi/reconciliation.yaml.
// Run: TEST_DATABASE_URL=... go test -tags openapi -run OpenAPI -v ./...

func TestOpenAPIListExceptionsSuccessMatchesContract(t *testing.T) {
	_, router := lab5SeededServer(t)
	lab5CheckAgainstSpec(t, router, http.MethodGet, "/api/v1/exceptions?limit=2")
}

func TestOpenAPIAccountExceptionsSuccessMatchesContract(t *testing.T) {
	_, router := lab5SeededServer(t)
	lab5CheckAgainstSpec(t, router, http.MethodGet, "/api/v1/accounts/a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11/exceptions")
}

func TestOpenAPIAccountNotFoundMatchesContract(t *testing.T) {
	_, router := lab5SeededServer(t)
	lab5CheckAgainstSpec(t, router, http.MethodGet, "/api/v1/accounts/00000000-0000-0000-0000-000000000000/exceptions")
}

func TestOpenAPITriggerJobMatchesContract(t *testing.T) {
	_, router := lab5SeededServer(t)
	lab5CheckAgainstSpec(t, router, http.MethodPost, "/api/v1/reconciliations/jobs")
}
