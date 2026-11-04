package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of the API response we will read.
const maxResponseBytes = 1 << 20

// Category is a bounded, non-sensitive classification of a poll failure. It is
// the only label used by the exporter and is safe to expose.
type Category string

const (
	CategoryAuth        Category = "auth"
	CategoryRateLimited Category = "rate_limited"
	CategoryHTTPStatus  Category = "http_status"
	CategoryDecode      Category = "decode"
	CategoryNetwork     Category = "network"
	CategoryConfig      Category = "config"
)

// Categories lists every failure category so the error counter can be
// pre-initialised with stable zero-valued series.
var Categories = []Category{
	CategoryAuth,
	CategoryRateLimited,
	CategoryHTTPStatus,
	CategoryDecode,
	CategoryNetwork,
	CategoryConfig,
}

// Credits is one successful reading of the account balance, in USD.
type Credits struct {
	TotalCredits float64
	TotalUsage   float64
}

// Remaining is the OpenRouter account credit still available, in USD.
func (c Credits) Remaining() float64 { return c.TotalCredits - c.TotalUsage }

// APIError describes a failed poll without carrying the key, the raw response
// body or any sensitive header value.
type APIError struct {
	Category   Category
	StatusCode int
	reason     string
	cause      error
}

func (e *APIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("category=%s status=%d reason=%s", e.Category, e.StatusCode, e.reason)
	}
	return fmt.Sprintf("category=%s reason=%s", e.Category, e.reason)
}

func (e *APIError) Unwrap() error { return e.cause }

// CategoryOf reports the failure category for err, defaulting to network for
// transport-level errors.
func CategoryOf(err error) Category {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Category
	}
	return CategoryNetwork
}

// Client talks to the OpenRouter credits endpoint.
type Client struct {
	apiURL string
	key    string
	http   *http.Client
}

// NewClient builds a client with a bounded HTTP timeout. A single attempt is
// made per poll; retries are deliberately absent.
func NewClient(apiURL, key string, timeout time.Duration) *Client {
	return &Client{
		apiURL: apiURL,
		key:    key,
		http: &http.Client{
			Timeout: timeout,
			// Never follow redirects: a management key must not be replayed
			// against an unexpected host.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

type creditsEnvelope struct {
	Data *struct {
		TotalCredits *float64 `json:"total_credits"`
		TotalUsage   *float64 `json:"total_usage"`
	} `json:"data"`
}

// FetchCredits performs exactly one GET against the credits endpoint.
func (c *Client) FetchCredits(ctx context.Context) (Credits, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return Credits{}, &APIError{Category: CategoryConfig, reason: "invalid API URL", cause: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "mono-openrouter-credits-exporter/1.0 (+https://github.com/solanyn/mono)")
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || os.IsTimeout(err) {
			return Credits{}, &APIError{Category: CategoryNetwork, reason: "request timed out", cause: err}
		}
		return Credits{}, &APIError{Category: CategoryNetwork, reason: "request failed", cause: err}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return Credits{}, &APIError{Category: CategoryAuth, StatusCode: resp.StatusCode, reason: "management key rejected"}
	case resp.StatusCode == http.StatusTooManyRequests:
		return Credits{}, &APIError{Category: CategoryRateLimited, StatusCode: resp.StatusCode, reason: "rate limited"}
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return Credits{}, &APIError{Category: CategoryHTTPStatus, StatusCode: resp.StatusCode, reason: "unexpected status"}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Credits{}, &APIError{Category: CategoryNetwork, StatusCode: resp.StatusCode, reason: "reading response failed", cause: err}
	}
	return parseCredits(body)
}

func parseCredits(body []byte) (Credits, error) {
	var env creditsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Credits{}, &APIError{Category: CategoryDecode, reason: "response is not valid JSON", cause: err}
	}
	if env.Data == nil || env.Data.TotalCredits == nil || env.Data.TotalUsage == nil {
		return Credits{}, &APIError{Category: CategoryDecode, reason: "response is missing data.total_credits or data.total_usage"}
	}
	credits := Credits{TotalCredits: *env.Data.TotalCredits, TotalUsage: *env.Data.TotalUsage}
	for _, v := range []float64{credits.TotalCredits, credits.TotalUsage} {
		if isNonFinite(v) {
			return Credits{}, &APIError{Category: CategoryDecode, reason: "response contains a non-finite value"}
		}
	}
	return credits, nil
}

// Redact is a defensive helper: any string that ends up in a log line or an
// HTTP response is passed through it so a leaked credential can never be
// echoed. It is a no-op for empty keys.
func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "[REDACTED]")
}
