package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-or-v1-TESTKEY-must-never-leak-0000"

func testClient(t *testing.T, handler http.Handler, timeout time.Duration) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, testKey, timeout)
}

func TestFetchCreditsValidResponse(t *testing.T) {
	var gotAuth, gotAccept string
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"total_credits":100.5,"total_usage":37.25}}`))
	}), 5*time.Second)

	credits, err := client.FetchCredits(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credits.TotalCredits != 100.5 || credits.TotalUsage != 37.25 {
		t.Fatalf("unexpected credits: %+v", credits)
	}
	if got := credits.Remaining(); got != 63.25 {
		t.Fatalf("remaining = %v, want 63.25", got)
	}
	if gotAuth != "Bearer "+testKey {
		t.Fatalf("authorization header not sent as bearer token")
	}
	if gotAccept != "application/json" {
		t.Fatalf("accept header = %q", gotAccept)
	}
}

func TestRemainingCalculationHandlesZeroAndExhaustion(t *testing.T) {
	cases := []struct {
		name     string
		credits  Credits
		expected float64
	}{
		{"full balance", Credits{TotalCredits: 10, TotalUsage: 0}, 10},
		{"exhausted", Credits{TotalCredits: 10, TotalUsage: 10}, 0},
		{"overdrawn", Credits{TotalCredits: 10, TotalUsage: 12.5}, -2.5},
		{"zero account", Credits{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.credits.Remaining(); got != tc.expected {
				t.Fatalf("remaining = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFetchCreditsNon2xx(t *testing.T) {
	cases := []struct {
		status   int
		category Category
	}{
		{http.StatusUnauthorized, CategoryAuth},
		{http.StatusForbidden, CategoryAuth},
		{http.StatusTooManyRequests, CategoryRateLimited},
		{http.StatusInternalServerError, CategoryHTTPStatus},
		{http.StatusNotFound, CategoryHTTPStatus},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"error":"sensitive upstream detail"}`))
			}), 5*time.Second)

			_, err := client.FetchCredits(context.Background())
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := CategoryOf(err); got != tc.category {
				t.Fatalf("category = %q, want %q", got, tc.category)
			}
			assertNoSecretLeak(t, err.Error())
		})
	}
}

func TestFetchCreditsMalformedAndPartialResponse(t *testing.T) {
	cases := map[string]string{
		"not json":       `<html>upstream gateway</html>`,
		"empty body":     ``,
		"missing data":   `{"meta":{}}`,
		"null data":      `{"data":null}`,
		"missing usage":  `{"data":{"total_credits":5}}`,
		"missing totals": `{"data":{"total_usage":5}}`,
		"wrong types":    `{"data":{"total_credits":"five","total_usage":1}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}), 5*time.Second)

			_, err := client.FetchCredits(context.Background())
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := CategoryOf(err); got != CategoryDecode {
				t.Fatalf("category = %q, want %q", got, CategoryDecode)
			}
			assertNoSecretLeak(t, err.Error())
		})
	}
}

func TestFetchCreditsTimeout(t *testing.T) {
	release := make(chan struct{})
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}), 100*time.Millisecond)
	defer close(release)

	start := time.Now()
	_, err := client.FetchCredits(context.Background())
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout was not bounded, elapsed %s", elapsed)
	}
	if got := CategoryOf(err); got != CategoryNetwork {
		t.Fatalf("category = %q, want %q", got, CategoryNetwork)
	}
	assertNoSecretLeak(t, err.Error())
}

func TestFetchCreditsContextCancellation(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}), 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FetchCredits(ctx); err == nil {
		t.Fatal("expected an error for a cancelled context")
	} else if got := CategoryOf(err); got != CategoryNetwork {
		t.Fatalf("category = %q, want %q", got, CategoryNetwork)
	}
}

func TestFetchCreditsDoesNotFollowRedirect(t *testing.T) {
	var replayed bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replayed = r.Header.Get("Authorization") != ""
		_, _ = w.Write([]byte(`{"data":{"total_credits":1,"total_usage":1}}`))
	}))
	defer target.Close()

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}), 5*time.Second)

	_, err := client.FetchCredits(context.Background())
	if err == nil {
		t.Fatal("expected the redirect to surface as an error")
	}
	if got := CategoryOf(err); got != CategoryHTTPStatus {
		t.Fatalf("category = %q, want %q", got, CategoryHTTPStatus)
	}
	if replayed {
		t.Fatal("management key was replayed against the redirect target")
	}
}

func TestFetchCreditsRejectsOversizedResponse(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		big := strings.Repeat("A", maxResponseBytes+1024)
		_, _ = w.Write([]byte(`{"data":{"total_credits":1,"total_usage":1},"pad":"` + big + `"}`))
	}), 10*time.Second)

	if _, err := client.FetchCredits(context.Background()); err == nil {
		t.Fatal("expected an error for a truncated oversized response")
	}
}

func TestErrorStringsNeverContainSecrets(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid key ` + testKey + `"}}`))
	}), 5*time.Second)

	_, err := client.FetchCredits(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	assertNoSecretLeak(t, err.Error())
	if strings.Contains(err.Error(), "invalid key") {
		t.Fatalf("upstream response body leaked into the error: %q", err.Error())
	}
}

func TestRedactRemovesSecret(t *testing.T) {
	if got := redact("failed with "+testKey+" attached", testKey); strings.Contains(got, testKey) {
		t.Fatalf("secret survived redaction: %q", got)
	}
	if got := redact("no secret here", ""); got != "no secret here" {
		t.Fatalf("redact with empty secret altered the string: %q", got)
	}
}

// assertNoSecretLeak fails if a string carries the management key or the placeholder
// credential patterns that must never reach logs or HTTP responses.
func assertNoSecretLeak(t *testing.T, s string) {
	t.Helper()
	if strings.Contains(s, testKey) {
		t.Fatalf("management key leaked: %q", s)
	}
	if strings.Contains(s, "Bearer ") {
		t.Fatalf("authorization header leaked: %q", s)
	}
}
