package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// webhookRequest is the agentgateway Guardrail Webhook payload. The body is
// kept as a generic map so unknown fields round-trip unmodified.
type webhookRequest struct {
	Body map[string]interface{} `json:"body"`
}

// handleRequest scrubs PII from the messages of an inbound request body.
func (s *server) handleRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeWebhook(w, r)
	if !ok {
		return
	}

	messages, ok := req.Body["messages"].([]interface{})
	if !ok {
		writePass(w)
		return
	}

	changed := false
	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		content, exists := msg["content"]
		if !exists {
			continue
		}
		scrubbed, c, err := scrubContent(r.Context(), s.client, content)
		if err != nil {
			log.Printf("warning: scrub failed on request, failing open: %v", err)
			writePass(w)
			return
		}
		if c {
			msg["content"] = scrubbed
			changed = true
		}
	}

	if changed {
		writeMask(w, req.Body)
		return
	}
	writePass(w)
}

// handleResponse scrubs PII from the choices of an outbound response body.
func (s *server) handleResponse(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeWebhook(w, r)
	if !ok {
		return
	}

	choices, ok := req.Body["choices"].([]interface{})
	if !ok {
		writePass(w)
		return
	}

	changed := false
	for _, ch := range choices {
		choice, ok := ch.(map[string]interface{})
		if !ok {
			continue
		}
		msg, ok := choice["message"].(map[string]interface{})
		if !ok {
			continue
		}
		content, exists := msg["content"]
		if !exists {
			continue
		}
		scrubbed, c, err := scrubContent(r.Context(), s.client, content)
		if err != nil {
			log.Printf("warning: scrub failed on response, failing open: %v", err)
			writePass(w)
			return
		}
		if c {
			msg["content"] = scrubbed
			changed = true
		}
	}

	if changed {
		writeMask(w, req.Body)
		return
	}
	writePass(w)
}

// handleHealth reports adapter health based on Presidio's /health endpoint.
func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	analyzerUp := s.client.health(r.Context())
	resp := map[string]interface{}{"analyzer": analyzerUp}
	if analyzerUp {
		resp["status"] = "ok"
	} else {
		resp["status"] = "degraded"
	}
	writeJSON(w, resp)
}

// decodeWebhook decodes the request body. On any decode error or missing body
// it fails open by writing a pass response and returns ok=false.
func decodeWebhook(w http.ResponseWriter, r *http.Request) (webhookRequest, bool) {
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("warning: failed to decode webhook body, failing open: %v", err)
		writePass(w)
		return req, false
	}
	if req.Body == nil {
		writePass(w)
		return req, false
	}
	return req, true
}

// writePass emits the fail-open / no-change response.
// agentgateway expects an externally-tagged serde enum: {"action":{"pass":{}}}
func writePass(w http.ResponseWriter) {
	writeJSON(w, map[string]interface{}{
		"action": map[string]interface{}{
			"pass": map[string]interface{}{},
		},
	})
}

// writeMask emits a mask response carrying the modified body.
// agentgateway expects an externally-tagged serde enum:
// {"action":{"mask":{"body":{...}}}}
func writeMask(w http.ResponseWriter, body map[string]interface{}) {
	writeJSON(w, map[string]interface{}{
		"action": map[string]interface{}{
			"mask": map[string]interface{}{
				"body": body,
			},
		},
	})
}

// writeJSON writes v as a JSON response. Encoding failures are logged but not
// surfaced, keeping the adapter fail-open.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("warning: failed to encode response: %v", err)
	}
}
