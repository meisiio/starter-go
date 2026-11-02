package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestS2Client_FetchJournalEntries(t *testing.T) {
	testCursor := "cursor_page_2"
	refID := "0192a7e4-6b1d-7c3e-9a41-2f5d8e0b7c11"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/journal-entries" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		// Verify correlation header propagation
		cid := r.Header.Get("X-Correlation-ID")
		if cid != "test-cid-123" {
			t.Errorf("expected X-Correlation-ID test-cid-123, got: %s", cid)
		}

		// Verify query params
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("expected limit=50, got: %s", r.URL.Query().Get("limit"))
		}

		resp := PagedJournalEntries{
			Items: []JournalEntryDto{
				{
					ID:           "0192a7e4-0001-7c3e-9a41-000000000001",
					EntryNumber:  501,
					EntryType:    "TRANSFER_INTERNAL",
					ReferenceID:  &refID,
					TotalAmount:  25000000,
					PostedAt:     time.Now().UTC(),
					BusinessDate: "2026-10-31",
					Lines: []JournalLineDto{
						{LineNo: 1, AccountID: "acc-1", Direction: "DEBIT", Amount: 25000000, BalanceAfter: 75000000},
						{LineNo: 2, AccountID: "acc-2", Direction: "CREDIT", Amount: 25000000, BalanceAfter: 25000000},
					},
				},
			},
			NextCursor: &testCursor,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewS2Client(S2ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	ctx := context.WithValue(context.Background(), correlationKey, "test-cid-123")

	result, err := client.FetchJournalEntries(ctx, JournalEntryQuery{Limit: 50})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].TotalAmount != 25000000 {
		t.Errorf("expected total_amount 25000000, got: %d", result.Items[0].TotalAmount)
	}
	if result.NextCursor == nil || *result.NextCursor != testCursor {
		t.Errorf("expected next_cursor %s, got %v", testCursor, result.NextCursor)
	}
}

func TestS2Client_FetchTrialBalance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reports/trial-balance" {
			http.NotFound(w, r)
			return
		}

		resp := TrialBalanceResponse{
			AsOfDate:    "2026-10-31",
			TotalDebit:  100000000,
			TotalCredit: 100000000,
			IsBalanced:  true,
			Accounts: []TrialBalanceAccountDto{
				{AccountID: "acc-1", TotalDebits: 100000000, TotalCredits: 0, Balance: 100000000},
				{AccountID: "acc-2", TotalDebits: 0, TotalCredits: 100000000, Balance: -100000000},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewS2Client(S2ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	ctx := context.Background()

	tb, err := client.FetchTrialBalance(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !tb.IsBalanced {
		t.Errorf("expected is_balanced to be true")
	}
	if tb.TotalDebit != tb.TotalCredit {
		t.Errorf("expected debit == credit")
	}
}

func TestS2Client_FetchTrialBalance_Unbalanced(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := TrialBalanceResponse{
			AsOfDate:    "2026-10-31",
			TotalDebit:  100000000,
			TotalCredit: 95000000,
			IsBalanced:  false,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewS2Client(S2ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	_, err := client.FetchTrialBalance(context.Background())
	if err == nil {
		t.Fatalf("expected error for unbalanced trial balance, got nil")
	}
	if !errors.Is(err, ErrTrialBalanceFailed) {
		t.Errorf("expected ErrTrialBalanceFailed, got: %v", err)
	}
}

func TestS2Client_ProblemDetailsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prob := ProblemDetails{
			Type:   "https://errors.labbank.example/not-found",
			Title:  "Account Not Found",
			Status: http.StatusNotFound,
			Code:   "NOT_FOUND",
			Detail: "Account ID not found in ledger database",
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(prob)
	}))
	defer server.Close()

	client := NewS2Client(S2ClientConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	_, err := client.FetchJournalEntries(context.Background(), JournalEntryQuery{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var s2Err *S2ProblemError
	if !errors.As(err, &s2Err) {
		t.Fatalf("expected *S2ProblemError, got %T: %v", err, err)
	}
	if s2Err.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", s2Err.StatusCode)
	}
	if s2Err.Code != "NOT_FOUND" {
		t.Errorf("expected code NOT_FOUND, got %s", s2Err.Code)
	}
}
