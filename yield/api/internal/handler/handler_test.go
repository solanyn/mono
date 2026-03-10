package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/solanyn/mono/yield/api/internal/domain"
)

func TestHealth(t *testing.T) {
	h := New()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health() status = %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("Health() body = %v, want status=ok", body)
	}
}

func TestAnalyze(t *testing.T) {
	h := New()

	reqBody := domain.AnalyseRequest{
		Address:     "123 Test St",
		Price:       500000,
		RentPerWeek: 500,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Analyze() status = %d, want %d", w.Code, http.StatusOK)
	}

	var result domain.AnalyseResult
	json.NewDecoder(w.Body).Decode(&result)

	expectedYield := float64(500*52) / float64(500000) * 100
	if result.GrossYield != expectedYield {
		t.Errorf("Analyze() yield = %v, want %v", result.GrossYield, expectedYield)
	}
}

func TestAnalyzeBadRequest(t *testing.T) {
	h := New()
	req := httptest.NewRequest(http.MethodPost, "/api/analyze", bytes.NewReader([]byte("invalid")))
	w := httptest.NewRecorder()

	h.Analyze(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Analyze() status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
