package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWritePassFormat verifies the pass response is externally-tagged:
// {"action":{"pass":{}}}
func TestWritePassFormat(t *testing.T) {
	rr := httptest.NewRecorder()
	writePass(rr)

	resp := rr.Result()
	body, _ := io.ReadAll(resp.Body)

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("invalid JSON: %v", body)
	}

	// Must have exactly one key "action"
	keys := mapKeys(out)
	if len(keys) != 1 || keys[0] != "action" {
		t.Fatalf("expected single key \"action\", got %v", keys)
	}

	// action must be an object with exactly one key "pass"
	action, ok := out["action"].(map[string]any)
	if !ok {
		t.Fatalf("action is %T, want map", out["action"])
	}
	actionKeys := mapKeys(action)
	if len(actionKeys) != 1 || actionKeys[0] != "pass" {
		t.Fatalf("expected action to have key \"pass\", got %v", actionKeys)
	}

	// pass must be an empty object {}
	pass, ok := action["pass"].(map[string]any)
	if !ok {
		t.Fatalf("pass is %T, want map", action["pass"])
	}
	if len(pass) != 0 {
		t.Fatalf("expected empty pass {}, got %v", pass)
	}

	// Should NOT be the old flat format {"action":"pass"}
	if strings.Contains(string(body), `"action":"pass"`) {
		t.Fatalf("response uses flat string format, want externally-tagged: %s", body)
	}
}

// TestWriteMaskFormat verifies the mask response is externally-tagged:
// {"action":{"mask":{"body":{...}}}}
func TestWriteMaskFormat(t *testing.T) {
	body := map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "hi"},
		},
	}
	rr := httptest.NewRecorder()
	writeMask(rr, body)

	resp := rr.Result()
	responseBody, _ := io.ReadAll(resp.Body)

	var out map[string]any
	if err := json.Unmarshal(responseBody, &out); err != nil {
		t.Fatalf("invalid JSON: %v", responseBody)
	}

	// Must have exactly one key "action"
	keys := mapKeys(out)
	if len(keys) != 1 || keys[0] != "action" {
		t.Fatalf("expected single key \"action\", got %v", keys)
	}

	// action must be an object with exactly one key "mask"
	action, ok := out["action"].(map[string]any)
	if !ok {
		t.Fatalf("action is %T, want map", out["action"])
	}
	actionKeys := mapKeys(action)
	if len(actionKeys) != 1 || actionKeys[0] != "mask" {
		t.Fatalf("expected action to have key \"mask\", got %v", actionKeys)
	}

	// mask must be an object with a "body" key
	mask, ok := action["mask"].(map[string]any)
	if !ok {
		t.Fatalf("mask is %T, want map", action["mask"])
	}
	maskKeys := mapKeys(mask)
	if len(maskKeys) != 1 || maskKeys[0] != "body" {
		t.Fatalf("expected mask to have key \"body\", got %v", maskKeys)
	}

	// Should NOT be the old flat format {"action":"mask","body":{...}}
	if strings.Contains(string(responseBody), `"action":"mask"`) {
		t.Fatalf("response uses flat string format, want externally-tagged: %s", responseBody)
	}
}

// mapKeys returns the keys of a map[string]any.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// fakePresidio returns a presidioClient that returns the given results.
func fakePresidio(t *testing.T, results []analyzeResult) *presidioClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(results)
	}))
	t.Cleanup(srv.Close)
	return newPresidioClient(Config{PresidioURL: srv.URL, Language: "en", ScoreThreshold: 0.5})
}

// TestHandleRequestEndToEnd exercises the full /request handler with a fake
// Presidio, verifying the response format is externally-tagged.
func TestHandleRequestEndToEnd(t *testing.T) {
	client := fakePresidio(t, []analyzeResult{
		{EntityType: "PERSON", Start: 0, End: 4, Score: 0.9},
	})
	srv := &server{client: client}

	// POST /request with a body containing a message with PII
	body := `{"body":{"messages":[{"role":"user","content":"John went to the store"}]}}`
	req := httptest.NewRequest(http.MethodPost, "/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.handleRequest(rr, req)

	resp := rr.Result()
	respBody, _ := io.ReadAll(resp.Body)

	// Must be 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", resp.StatusCode, respBody)
	}

	// Parse response - should be externally-tagged mask
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("invalid JSON: %v", respBody)
	}

	action, ok := out["action"].(map[string]any)
	if !ok {
		t.Fatalf("action is %T, want map", out["action"])
	}
	mask, ok := action["mask"].(map[string]any)
	if !ok {
		t.Fatalf("action.mask missing, got keys %v", mapKeys(action))
	}
	_, ok = mask["body"].(map[string]any)
	if !ok {
		t.Fatalf("mask.body missing or wrong type, got %v", mask)
	}
}