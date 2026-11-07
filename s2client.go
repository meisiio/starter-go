package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultS2BaseURL is the default location of the S2 Ledger service.
const DefaultS2BaseURL = "http://localhost:8082"

// S2ClientConfig holds configuration for the S2 Ledger client.
type S2ClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// LoadS2Config loads S2 client settings from environment variables.
func LoadS2Config() S2ClientConfig {
	baseURL := os.Getenv("NEIGHBOR_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("S2_LEDGER_BASE_URL")
	}
	if baseURL == "" {
		baseURL = DefaultS2BaseURL
	}

	return S2ClientConfig{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Timeout: 5 * time.Second,
	}
}

// S2Client communicates with Service 2 (Ledger & Balance - S2-LDG).
type S2Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewS2Client initializes a new S2Client with connection pooling and timeouts.
func NewS2Client(cfg S2ClientConfig) *S2Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	return &S2Client{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// --- DTOs (Data Transfer Objects) matching S2 Specification ---

// JournalLineDto represents a single debit/credit line of a ledger entry.
type JournalLineDto struct {
	LineNo       int    `json:"line_no"`
	AccountID    string `json:"account_id"`
	Direction    string `json:"direction"` // DEBIT | CREDIT
	Amount       int64  `json:"amount"`    // In Rials (IRR)
	BalanceAfter int64  `json:"balance_after"`
}

// JournalEntryDto represents an official ledger journal entry.
type JournalEntryDto struct {
	ID            string           `json:"id"`
	EntryNumber   int64            `json:"entry_number"`
	EntryType     string           `json:"entry_type"` // TRANSFER_INTERNAL, TRANSFER_EXTERNAL, etc.
	ReferenceID   *string          `json:"reference_id,omitempty"`
	TotalAmount   int64            `json:"total_amount"`
	PostedAt      time.Time        `json:"posted_at"`
	BusinessDate  string           `json:"business_date"`
	CorrelationID string           `json:"correlation_id,omitempty"`
	Lines         []JournalLineDto `json:"lines"`
}

// JournalEntryQuery specifies query parameters for fetching journal entries.
type JournalEntryQuery struct {
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
	AccountID   string
	ReferenceID string
}

// PagedJournalEntries represents a paginated response from S2.
type PagedJournalEntries struct {
	Items      []JournalEntryDto `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// TrialBalanceAccountDto represents debit/credit summary for an account.
type TrialBalanceAccountDto struct {
	AccountID    string `json:"account_id"`
	TotalDebits  int64  `json:"total_debits"`
	TotalCredits int64  `json:"total_credits"`
	Balance      int64  `json:"balance"`
}

// TrialBalanceResponse represents trial balance integrity report from S2.
type TrialBalanceResponse struct {
	AsOfDate    string                   `json:"as_of_date"`
	TotalDebit  int64                    `json:"total_debit"`
	TotalCredit int64                    `json:"total_credit"`
	IsBalanced  bool                     `json:"is_balanced"`
	Accounts    []TrialBalanceAccountDto `json:"accounts"`
}

// --- Fault Tolerance & Error Handling ---

// S2ProblemError represents RFC 7807/9457 error details returned by S2.
type S2ProblemError struct {
	StatusCode int
	Code       string
	Title      string
	Detail     string
	Problem    ProblemDetails
}

func (e *S2ProblemError) Error() string {
	return fmt.Sprintf("s2 error (status %d, code %s): %s - %s", e.StatusCode, e.Code, e.Title, e.Detail)
}

// Client error categories
var (
	ErrUpstreamTimeout    = errors.New("UPSTREAM_TIMEOUT: upstream S2 service request timed out")
	ErrServiceUnavailable = errors.New("SERVICE_UNAVAILABLE: upstream S2 service is unreachable")
	ErrTrialBalanceFailed = errors.New("TRIAL_BALANCE_UNBALANCED: S2 ledger total debits do not equal total credits")
)

func (c *S2Client) handleRequestError(err error) error {
	if err == nil {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
	}
	return fmt.Errorf("%w: %v", ErrServiceUnavailable, err)
}

func (c *S2Client) doRequest(req *http.Request) (*http.Response, error) {
	// Propagate X-Correlation-ID from context (GEN-08)
	if cid, ok := req.Context().Value(correlationKey).(string); ok && cid != "" {
		req.Header.Set("X-Correlation-ID", cid)
	}
	req.Header.Set("Accept", "application/json, application/problem+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, c.handleRequestError(err)
	}
	return resp, nil
}

// --- Client Methods ---

// FetchJournalEntries queries S2 for ledger entries matching the query criteria.
// Target: GET /api/v1/journal-entries
func (c *S2Client) FetchJournalEntries(ctx context.Context, query JournalEntryQuery) (*PagedJournalEntries, error) {
	endpoint := fmt.Sprintf("%s/api/v1/journal-entries", c.baseURL)

	params := url.Values{}
	if query.From != nil {
		params.Set("from", query.From.UTC().Format(time.RFC3339))
	}
	if query.To != nil {
		params.Set("to", query.To.UTC().Format(time.RFC3339))
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	params.Set("limit", strconv.Itoa(limit))

	if query.Cursor != "" {
		params.Set("cursor", query.Cursor)
	}
	if query.AccountID != "" {
		params.Set("account_id", query.AccountID)
	}
	if query.ReferenceID != "" {
		params.Set("reference_id", query.ReferenceID)
	}

	if len(params) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var prob ProblemDetails
		if err := json.Unmarshal(bodyBytes, &prob); err == nil && prob.Title != "" {
			return nil, &S2ProblemError{
				StatusCode: resp.StatusCode,
				Code:       prob.Code,
				Title:      prob.Title,
				Detail:     prob.Detail,
				Problem:    prob,
			}
		}
		return nil, &S2ProblemError{
			StatusCode: resp.StatusCode,
			Code:       "UPSTREAM_ERROR",
			Title:      "Upstream S2 Error",
			Detail:     string(bodyBytes),
		}
	}

	var result PagedJournalEntries
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal paged journal entries: %w", err)
	}

	return &result, nil
}

// FetchAllJournalEntries collects all journal entries in a date range by following cursor pagination.
func (c *S2Client) FetchAllJournalEntries(ctx context.Context, query JournalEntryQuery) ([]JournalEntryDto, error) {
	var allEntries []JournalEntryDto
	currentQuery := query

	for {
		page, err := c.FetchJournalEntries(ctx, currentQuery)
		if err != nil {
			return nil, err
		}

		allEntries = append(allEntries, page.Items...)

		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		currentQuery.Cursor = *page.NextCursor
	}

	return allEntries, nil
}

// FetchTrialBalance queries S2 to verify ledger integrity (sum debits == sum credits).
// Target: GET /api/v1/reports/trial-balance
func (c *S2Client) FetchTrialBalance(ctx context.Context) (*TrialBalanceResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/reports/trial-balance", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var prob ProblemDetails
		if err := json.Unmarshal(bodyBytes, &prob); err == nil && prob.Title != "" {
			return nil, &S2ProblemError{
				StatusCode: resp.StatusCode,
				Code:       prob.Code,
				Title:      prob.Title,
				Detail:     prob.Detail,
				Problem:    prob,
			}
		}
		return nil, &S2ProblemError{
			StatusCode: resp.StatusCode,
			Code:       "UPSTREAM_ERROR",
			Title:      "Upstream S2 Error",
			Detail:     string(bodyBytes),
		}
	}

	var tb TrialBalanceResponse
	if err := json.Unmarshal(bodyBytes, &tb); err != nil {
		return nil, fmt.Errorf("unmarshal trial balance: %w", err)
	}

	// Verify integrity checks
	if !tb.IsBalanced || tb.TotalDebit != tb.TotalCredit {
		return &tb, fmt.Errorf("%w: total_debit=%d total_credit=%d is_balanced=%t",
			ErrTrialBalanceFailed, tb.TotalDebit, tb.TotalCredit, tb.IsBalanced)
	}

	return &tb, nil
}
