package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// ---- Consumer contract test against the neighbour's REAL service (Lab 5, section 9) ----
//
//	NEIGHBOR_BASE_URL=http://<neighbour-ip>:8082 go test -run Contract -v -count=1 ./...
//
// Without NEIGHBOR_BASE_URL the tests are skipped. Align the paths and the error case with
// the neighbour's OpenAPI file; every log line carries a timestamp for the report.

func lab5NeighborURL(t *testing.T) string {
	t.Helper()
	base := strings.TrimRight(os.Getenv("NEIGHBOR_BASE_URL"), "/")
	if base == "" {
		t.Skip("NEIGHBOR_BASE_URL is not set: skipping the contract test against the neighbour")
	}
	return base
}

func lab5Stamp() string { return time.Now().UTC().Format(time.RFC3339) }

// Success path: the neighbour's list endpoint answers 200 and our client can parse it.
func TestContractNeighborJournalEntriesSuccess(t *testing.T) {
	base := lab5NeighborURL(t)
	t.Logf("[%s] GET %s/api/v1/journal-entries?limit=1", lab5Stamp(), base)

	client := NewS2Client(S2ClientConfig{BaseURL: base, Timeout: 5 * time.Second})
	page, err := client.FetchJournalEntries(context.Background(), JournalEntryQuery{Limit: 1})
	t.Logf("[%s] finished, error=%v", lab5Stamp(), err)
	if err != nil {
		t.Fatalf("the neighbour broke the contract or is unreachable: %v", err)
	}
	if page == nil || len(page.Items) > 1 {
		t.Errorf("limit=1 must return at most one item, got %+v", page)
	}
}

// Error path: an invalid query must come back as RFC 9457 Problem Details.
func TestContractNeighborErrorIsProblemDetails(t *testing.T) {
	base := lab5NeighborURL(t)
	target := base + "/api/v1/journal-entries?from=not-a-date"
	t.Logf("[%s] GET %s", lab5Stamp(), target)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(target)
	if err != nil {
		t.Fatalf("neighbour unreachable: %v", err)
	}
	defer resp.Body.Close()
	t.Logf("[%s] status=%d content-type=%q", lab5Stamp(), resp.StatusCode, resp.Header.Get("Content-Type"))

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	for _, field := range []string{"title", "status", "code"} {
		if _, ok := p[field]; !ok {
			t.Errorf("problem body lacks %q: %v", field, p)
		}
	}
}
