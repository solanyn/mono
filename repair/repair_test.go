package repair

import (
	"encoding/json"
	"testing"
)

func msg(role string, fields map[string]any) map[string]any {
	m := map[string]any{"role": role}
	for k, v := range fields {
		m[k] = v
	}
	return m
}

func req(messages ...map[string]any) []byte {
	b, _ := json.Marshal(map[string]any{
		"model":       "test-model",
		"stream":      false,
		"temperature": 0.7,
		"messages":    messages,
	})
	return b
}

func parseBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	return m
}

func getMessages(body []byte) []any {
	var m map[string]any
	json.Unmarshal(body, &m)
	msgs, _ := m["messages"].([]any)
	return msgs
}

// Case 2: tool message missing tool_call_id entirely (DeepSeek).
func TestRepairCase2_MissingToolCallID(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "ok"}),
		msg("tool", map[string]any{"content": "result"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	// Tool message should now have a synthetic tool_call_id.
	toolMsg := msgs[2].(map[string]any)
	id, ok := toolMsg["tool_call_id"].(string)
	if !ok || id == "" {
		t.Fatal("tool message missing tool_call_id after repair")
	}

	// Assistant should have tool_calls grafted.
	asst := msgs[1].(map[string]any)
	tcs, ok := asst["tool_calls"].([]any)
	if !ok || len(tcs) == 0 {
		t.Fatal("assistant missing tool_calls after repair")
	}
	tc := tcs[0].(map[string]any)
	if tc["id"] != id {
		t.Fatalf("tool_call id mismatch: got %v, want %v", tc["id"], id)
	}
}

// Case 1: tool message HAS tool_call_id but assistant missing matching tool_calls (MiniMax).
func TestRepairCase1_MissingToolCalls(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "ok"}),
		msg("tool", map[string]any{"tool_call_id": "call_abc", "content": "result"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	// Tool message should still have its original tool_call_id.
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["tool_call_id"] != "call_abc" {
		t.Fatal("tool_call_id was modified unexpectedly")
	}

	// Assistant should now have tool_calls (synthetic: unknown function).
	asst := msgs[1].(map[string]any)
	tcs, ok := asst["tool_calls"].([]any)
	if !ok || len(tcs) == 0 {
		t.Fatal("assistant missing tool_calls after repair")
	}
	tc := tcs[0].(map[string]any)
	if tc["id"] != "call_abc" {
		t.Fatalf("tool_call id mismatch: got %v, want call_abc", tc["id"])
	}
	fn := tc["function"].(map[string]any)
	if fn["name"] != "unknown" {
		t.Fatalf("expected synthetic name 'unknown', got %v", fn["name"])
	}
}

// Case 1 with cache: tool_calls grafted from cached real definition.
func TestRepairCase1_CacheGraft(t *testing.T) {
	cache := NewEngine()
	cache.mu.Lock()
	cache.cache["call_real"] = ToolCallDef{
		ID:   "call_real",
		Type: "function",
		Function: ToolCallFunctionDef{
			Name:      "get_weather",
			Arguments: `{"city":"Sydney"}`,
		},
	}
	cache.mu.Unlock()

	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "ok"}),
		msg("tool", map[string]any{"tool_call_id": "call_real", "content": "sunny"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	asst := msgs[1].(map[string]any)
	tcs := asst["tool_calls"].([]any)
	tc := tcs[0].(map[string]any)
	fn := tc["function"].(map[string]any)
	if fn["name"] != "get_weather" {
		t.Fatalf("expected cached name 'get_weather', got %v", fn["name"])
	}
}

// Both cases combined: missing tool_call_id AND missing tool_calls.
func TestRepair_BothCases(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "ok"}),
		msg("tool", map[string]any{"content": "result1"}),
		msg("tool", map[string]any{"tool_call_id": "call_known", "content": "result2"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	// First tool msg should get synthetic ID.
	t1 := msgs[2].(map[string]any)
	synthID, ok := t1["tool_call_id"].(string)
	if !ok || synthID == "" {
		t.Fatal("first tool message missing tool_call_id")
	}

	// Second tool msg should preserve its ID.
	t2 := msgs[3].(map[string]any)
	if t2["tool_call_id"] != "call_known" {
		t.Fatal("second tool_call_id was modified")
	}

	// Assistant should have tool_calls for both.
	asst := msgs[1].(map[string]any)
	tcs := asst["tool_calls"].([]any)
	if len(tcs) != 2 {
		t.Fatalf("expected 2 tool_calls, got %d", len(tcs))
	}
}

// Idempotency: already-correct messages pass through unchanged.
func TestRepair_Idempotent(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{
			"content": "ok",
			"tool_calls": []any{
				map[string]any{
					"id":   "call_abc",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": "{}",
					},
				},
			},
		}),
		msg("tool", map[string]any{"tool_call_id": "call_abc", "content": "sunny"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	// Tool message should be unchanged.
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["tool_call_id"] != "call_abc" {
		t.Fatal("existing tool_call_id was modified")
	}

	// Assistant should have exactly 1 tool_call (no duplicates).
	asst := msgs[1].(map[string]any)
	tcs := asst["tool_calls"].([]any)
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %d", len(tcs))
	}
}

// Top-level fields (model, stream, temperature, tools) preserved after repair.
func TestRepair_PreservesTopLevelFields(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "ok"}),
		msg("tool", map[string]any{"tool_call_id": "call_abc", "content": "result"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	parsed := parseBody(t, result)

	if parsed["model"] != "test-model" {
		t.Fatalf("model field lost: got %v", parsed["model"])
	}
	if parsed["stream"] != false {
		t.Fatalf("stream field lost: got %v", parsed["stream"])
	}
	if parsed["temperature"] != 0.7 {
		t.Fatalf("temperature field lost: got %v", parsed["temperature"])
	}
	if parsed["messages"] == nil {
		t.Fatal("messages field lost")
	}
}

// reasoning_content preserved for DeepSeek-style messages.
func TestRepair_PreservesReasoningContent(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{
			"content":           "ok",
			"reasoning_content": "I will call a tool",
		}),
		msg("tool", map[string]any{"tool_call_id": "call_xyz", "content": "done"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	asst := msgs[1].(map[string]any)
	if asst["reasoning_content"] != "I will call a tool" {
		t.Fatal("reasoning_content was lost or modified")
	}
	if asst["tool_calls"] == nil {
		t.Fatal("tool_calls not grafted despite reasoning_content presence")
	}
}

// Non-chat-completion requests pass through unchanged.
func TestRepair_NonChatPassthrough(t *testing.T) {
	cache := NewEngine()
	body := []byte(`{"prompt":"hello","stream":true}`)

	result := Repair(body, cache)
	if string(result) != string(body) {
		t.Fatalf("non-chat body was modified: got %s", result)
	}
}

// No tool messages = no-op (messages pass through unchanged).
func TestRepair_NoTools(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", map[string]any{"content": "hello"}),
		msg("assistant", map[string]any{"content": "hi"}),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)
	if len(msgs) != 2 {
		t.Fatalf("message count changed: got %d, want 2", len(msgs))
	}
	asst := msgs[1].(map[string]any)
	if asst["content"] != "hi" {
		t.Fatal("assistant content was modified")
	}
}

// Large body: repair handles big payloads.
func TestRepair_LargeBody(t *testing.T) {
	cache := NewEngine()
	msgs := make([]map[string]any, 50)
	for i := 0; i < 50; i++ {
		switch i % 4 {
		case 0:
			msgs[i] = msg("user", map[string]any{"content": "msg"})
		case 1:
			msgs[i] = msg("assistant", map[string]any{
				"content":           "ok",
				"reasoning_content": "some long reasoning text here",
			})
		case 2:
			// Insert some broken tool messages (missing tool_call_id).
			msgs[i] = msg("tool", map[string]any{"content": "result"})
		case 3:
			msgs[i] = msg("tool", map[string]any{
				"tool_call_id": "call_xyz",
				"content":      "result",
			})
		}
	}

	body, _ := json.Marshal(map[string]any{
		"model":    "deepseek-v4-pro",
		"stream":   false,
		"messages": msgs,
	})

	result := Repair(body, cache)
	parsed := parseBody(t, result)
	if parsed["model"] != "deepseek-v4-pro" {
		t.Fatal("model field lost in large body")
	}
	if len(getMessages(result)) != 50 {
		t.Fatal("message count changed in large body")
	}

	// Verify broken tool messages got tool_call_id.
	for i := 0; i < 50; i++ {
		if i%4 == 2 {
			tm := getMessages(result)[i].(map[string]any)
			if _, ok := tm["tool_call_id"]; !ok {
				t.Fatalf("tool message at index %d still missing tool_call_id", i)
			}
		}
	}
}

// Multiple assistant blocks with tool messages in between.
func TestRepair_MultipleAssistantBlocks(t *testing.T) {
	cache := NewEngine()
	body := req(
		msg("user", nil),
		msg("assistant", map[string]any{"content": "first"}),
		msg("tool", map[string]any{"content": "r1"}),
		msg("assistant", map[string]any{"content": "second"}),
		msg("tool", map[string]any{"tool_call_id": "call_2", "content": "r2"}),
		msg("user", nil),
	)

	result := Repair(body, cache)
	msgs := getMessages(result)

	// First assistant block should have tool_calls for tool at index 2.
	a1 := msgs[1].(map[string]any)
	tcs1 := a1["tool_calls"].([]any)
	if len(tcs1) != 1 {
		t.Fatalf("first assistant: expected 1 tool_call, got %d", len(tcs1))
	}

	// Second assistant block should have tool_calls for tool at index 4.
	a2 := msgs[3].(map[string]any)
	tcs2 := a2["tool_calls"].([]any)
	if len(tcs2) != 1 {
		t.Fatalf("second assistant: expected 1 tool_call, got %d", len(tcs2))
	}
	// Verify correct ID.
	if tcs2[0].(map[string]any)["id"] != "call_2" {
		t.Fatal("second assistant tool_call has wrong id")
	}
}
