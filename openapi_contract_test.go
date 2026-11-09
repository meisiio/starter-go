//go:build openapi

package main

// Validates our REAL responses against openapi/reconciliation.yaml (Lab 5, section 4).
// Opt-in so the normal build needs no extra dependency:
//
//	go get github.com/getkin/kin-openapi@latest && go mod tidy
//	go test -tags openapi -run OpenAPI -v ./...
//
// A failure here means the code and the contract disagree: fix one side and write down
// which one in the report ("is there a mismatch?").

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

func lab5CheckAgainstSpec(t *testing.T, h http.Handler, method, path string) {
	t.Helper()
	ctx := context.Background()

	doc, err := openapi3.NewLoader().LoadFromFile("openapi/reconciliation.yaml")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("the OpenAPI file itself is invalid: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	req := httptest.NewRequest(method, "http://localhost:8086"+path, nil)
	route, pathParams, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("%s %s is not described in the OpenAPI file: %v", method, path, err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route},
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(ctx, input); err != nil {
		t.Errorf("%s %s -> %d does not match the contract: %v\nbody: %s", method, path, rec.Code, err, rec.Body.String())
	}
}

func TestOpenAPIHealthMatchesContract(t *testing.T) {
	lab5CheckAgainstSpec(t, lab5Router(lab5NoDBServer()), http.MethodGet, "/health")
}

func TestOpenAPIListExceptionsErrorMatchesContract(t *testing.T) {
	lab5CheckAgainstSpec(t, lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/exceptions?limit=201")
}

func TestOpenAPIAccountExceptionsErrorMatchesContract(t *testing.T) {
	lab5CheckAgainstSpec(t, lab5Router(lab5NoDBServer()), http.MethodGet, "/api/v1/accounts/not-a-uuid/exceptions")
}
