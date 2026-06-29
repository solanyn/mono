package repair

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "repair_requests_total",
		Help: "POST /v1/chat/completions intercepted",
	})
	toolCallsGrafted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "repair_tool_calls_grafted",
		Help: "Case 1: tool_calls entries grafted into assistant messages",
	}, []string{"source"})
	toolIDsAdded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "repair_tool_ids_added",
		Help: "Case 2: synthetic tool_call_ids added to tool messages",
	})
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "repair_cache_hits",
		Help: "Cache lookup hits",
	})
	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "repair_cache_misses",
		Help: "Cache lookup misses",
	})
	bodySizeBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "repair_body_size_bytes",
		Help:    "Body sizes processed",
		Buckets: prometheus.ExponentialBuckets(256, 2, 16),
	}, []string{"direction"})
	durationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "repair_duration_seconds",
		Help:    "Processing latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"type"})
	parseErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "repair_parse_errors",
		Help: "JSON parse failures",
	}, []string{"direction"})
)

type ToolCallFunctionDef struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCallDef struct {
	ID       string              `json:"id"`
	Type     string              `json:"type"`
	Function ToolCallFunctionDef `json:"function"`
}

type Engine struct {
	mu    sync.RWMutex
	cache map[string]ToolCallDef
}

func NewEngine() *Engine {
	return &Engine{
		cache: make(map[string]ToolCallDef),
	}
}

type rawMsg map[string]any

func getStr(m rawMsg, key string) string {
	v, _ := m[key].(string)
	return v
}

func getSlice(m rawMsg, key string) []any {
	v, _ := m[key].([]any)
	return v
}

func repair(messages []rawMsg, cache *Engine) {
	for i, msg := range messages {
		if getStr(msg, "role") != "assistant" {
			continue
		}

		j := i + 1
		var toolMsgs []struct {
			idx int
			m   rawMsg
		}
		for j < len(messages) && getStr(messages[j], "role") == "tool" {
			toolMsgs = append(toolMsgs, struct {
				idx int
				m   rawMsg
			}{j, messages[j]})
			j++
		}
		if len(toolMsgs) == 0 {
			continue
		}

		for _, tm := range toolMsgs {
			_, hasID := tm.m["tool_call_id"]
			if !hasID {
				synth := "call_repair_" + strconv.FormatUint(uint64(rand.Uint32()), 16)
				tm.m["tool_call_id"] = synth
				toolIDsAdded.Inc()
			}
		}

		var existingIDs []string
		for _, tcRaw := range getSlice(msg, "tool_calls") {
			tc, ok := tcRaw.(map[string]any)
			if !ok {
				continue
			}
			if id, ok := tc["id"].(string); ok {
				existingIDs = append(existingIDs, id)
			}
		}

		var neededIDs []string
		for _, tm := range toolMsgs {
			if id := getStr(tm.m, "tool_call_id"); id != "" {
				neededIDs = append(neededIDs, id)
			}
		}

		for _, tid := range neededIDs {
			if slices.Contains(existingIDs, tid) {
				continue
			}
			cache.mu.RLock()
			def, ok := cache.cache[tid]
			cache.mu.RUnlock()
			if ok {
				cacheHits.Inc()
			} else {
				cacheMisses.Inc()
				def = ToolCallDef{
					ID:   tid,
					Type: "function",
					Function: ToolCallFunctionDef{
						Name:      "unknown",
						Arguments: "{}",
					},
				}
				toolCallsGrafted.WithLabelValues("synthetic").Inc()
			}

			toolCallsRaw := msg["tool_calls"]
			var tcs []any
			if toolCallsRaw != nil {
				tcs = toolCallsRaw.([]any)
			}

			found := false
			for _, tcRaw := range tcs {
				tc := tcRaw.(map[string]any)
				if tc["id"] == tid {
					found = true
					break
				}
			}
			if !found {
				if ok {
					toolCallsGrafted.WithLabelValues("cache").Inc()
				}
				tcs = append(tcs, map[string]any{
					"id":       def.ID,
					"type":     def.Type,
					"function": map[string]any{
						"name":      def.Function.Name,
						"arguments": def.Function.Arguments,
					},
				})
				msg["tool_calls"] = tcs
			}
		}
	}
}

func Repair(body []byte, cache *Engine) []byte {
	bodySizeBytes.WithLabelValues("request").Observe(float64(len(body)))
	requestsTotal.Inc()

	timer := prometheus.NewTimer(durationSeconds.WithLabelValues("request"))
	defer timer.ObserveDuration()

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		parseErrors.WithLabelValues("request").Inc()
		return body
	}

	messages, ok := req["messages"].([]any)
	if !ok {
		return body
	}

	rawMsgs := make([]rawMsg, len(messages))
	for i, m := range messages {
		rawMsgs[i] = m.(map[string]any)
	}
	repair(rawMsgs, cache)

	repairedMsgs := make([]any, len(rawMsgs))
	for i, m := range rawMsgs {
		repairedMsgs[i] = m
	}
	req["messages"] = repairedMsgs

	out, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return out
}

func CacheToolCalls(body []byte, cache *Engine) {
	bodySizeBytes.WithLabelValues("response").Observe(float64(len(body)))

	timer := prometheus.NewTimer(durationSeconds.WithLabelValues("response"))
	defer timer.ObserveDuration()

	cacheToolCallsJSON(body, cache)
}

func cacheToolCallsJSON(body []byte, cache *Engine) {
	if len(body) == 0 {
		return
	}

	var resp struct {
		Choices []struct {
			Message rawMsg `json:"message"`
			Delta   rawMsg `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		parseErrors.WithLabelValues("response").Inc()
		return
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	for _, choice := range resp.Choices {
		extractToolCalls(choice.Message, cache)
		extractToolCalls(choice.Delta, cache)
	}
}

func extractToolCalls(msg rawMsg, cache *Engine) {
	for _, tcRaw := range getSlice(msg, "tool_calls") {
		tc, ok := tcRaw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := tc["id"].(string)
		if id == "" {
			continue
		}
		typ, _ := tc["type"].(string)
		if typ == "" {
			typ = "function"
		}
		fnRaw, _ := tc["function"].(map[string]any)
		fn := ToolCallFunctionDef{Name: "unknown", Arguments: "{}"}
		if fnRaw != nil {
			if n, ok := fnRaw["name"].(string); ok {
				fn.Name = n
			}
			if a, ok := fnRaw["arguments"].(string); ok {
				fn.Arguments = a
			}
		}
		cache.cache[id] = ToolCallDef{
			ID:       id,
			Type:     typ,
			Function: fn,
		}
	}
}
