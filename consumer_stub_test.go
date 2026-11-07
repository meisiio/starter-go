package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// ---- Consumer side with a Stub (Lab 5, section 8) -----------------------------------
//
// The neighbour is simulated with httptest. The responses follow the neighbour's OpenAPI
// (S2 Ledger: GET /api/v1/journal-entries). We assert what OUR client does with them,
// not that the stub was called.

// lab5NeighborClient builds the client from NEIGHBOR_BASE_URL (falling back to
// S2_LEDGER_BASE_URL through LoadS2Config), like the running service does.
func lab5NeighborClient(timeout time.Duration) *S2Client {
	cfg := LoadS2Config()
	if base := strings.TrimRight(os.Getenv("NEIGHBOR_BASE_URL"), "/"); base != "" {
		cfg.BaseURL = base
	}
	cfg.Timeout = timeout
	return NewS2Client(cfg)
}

const lab5JournalPage = `{
  "items": [{
    "id": "0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11",
    "entry_number": 1,
    "entry_type": "TRANSFER_EXTERNAL",
    "reference_id": "f1111111-1111-1111-1111-111111111111",
    "total_amount": 2500000,
    "posted_at": "2026-11-01T10:00:00Z",
    "business_date": "2026-11-01",
    "lines": [{"line_no": 1, "account_id": "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
               "direction": "DEBIT", "amount": 2500000, "balance_after": 7500000}]
  }],
  "next_cursor": null
}`

func TestStubNeighborSuccessIsParsed(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/journal-entries" || r.URL.Query().Get("limit") != "10" {
			t.Errorf("unexpected request to the stub: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(lab5JournalPage))
	}))
	defer stub.Close()
	t.Setenv("NEIGHBOR_BASE_URL", stub.URL)

	page, err := lab5NeighborClient(2*time.Second).FetchJournalEntries(context.Background(), JournalEntryQuery{Limit: 10})
	if err != nil {
		t.Fatalf("FetchJournalEntries: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].TotalAmount != 2500000 {
		t.Fatalf("unexpected items: %+v", page.Items)
	}
	if got := page.Items[0].Lines[0].Direction; got != "DEBIT" {
		t.Errorf("line direction = %q, want DEBIT", got)
	}
	if page.NextCursor != nil {
		t.Errorf("next_cursor = %v, want nil", *page.NextCursor)
	}
}

func TestStubNeighborProblemDetailsBecomesTypedError(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"https://errors.labbank.example/service-unavailable",` +
			`"title":"Service unavailable","status":503,"code":"SERVICE_UNAVAILABLE","detail":"ledger database is down"}`))
	}))
	defer stub.Close()
	t.Setenv("NEIGHBOR_BASE_URL", stub.URL)

	_, err := lab5NeighborClient(2*time.Second).FetchJournalEntries(context.Background(), JournalEntryQuery{})

	var pe *S2ProblemError
	if !errors.As(err, &pe) {
		t.Fatalf("error = %v, want *S2ProblemError", err)
	}
	if pe.StatusCode != http.StatusServiceUnavailable || pe.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("unexpected problem: status=%d code=%s", pe.StatusCode, pe.Code)
	}
}

func TestStubNeighborSlowResponseIsAnUpstreamTimeout(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer stub.Close()
	t.Setenv("NEIGHBOR_BASE_URL", stub.URL)

	_, err := lab5NeighborClient(100*time.Millisecond).FetchJournalEntries(context.Background(), JournalEntryQuery{})
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Errorf("error = %v, want ErrUpstreamTimeout", err)
	}
}

func TestStubNeighborDownIsServiceUnavailable(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Setenv("NEIGHBOR_BASE_URL", stub.URL)
	stub.Close() // nothing listens on that address any more

	_, err := lab5NeighborClient(time.Second).FetchJournalEntries(context.Background(), JournalEntryQuery{})
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("error = %v, want ErrServiceUnavailable", err)
	}
}
