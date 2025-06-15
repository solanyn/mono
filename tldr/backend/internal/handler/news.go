package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	// newspb "tldr/gen/proto"
	"github.com/solanyn/goyangi/tldr/backend/internal/storage"

	"github.com/go-chi/chi/v5"
	// "github.com/go-chi/httplog/v3" // TODO: Re-enable when httplog v3 usage is fixed
)

type NewsHandler struct {
	storage *storage.Client
}

func NewNewsHandler(s *storage.Client) *NewsHandler {
	return &NewsHandler{storage: s}
}

func (h *NewsHandler) GetNews(w http.ResponseWriter, r *http.Request) {
	// TODO: Fix httplog v3 API usage
	// logger := httplog.LogEntry(r.Context())
	id := chi.URLParam(r, "id")
	key := fmt.Sprintf("news/%s.md", id)

	// logger.Info(fmt.Sprintf("Fetching news object with key: %s", key))
	obj, err := h.storage.GetObject(context.Background(), key)
	if err != nil {
		http.Error(w, `{"error": "Not found"}`, http.StatusNotFound)
		return
	}
	defer obj.Close()

	// logger.Info(fmt.Sprintf("Reading news object with key: %s", key))
	data, err := io.ReadAll(obj)
	if err != nil {
		http.Error(w, `{"error": "Failed to read file"}`, http.StatusInternalServerError)
		return
	}

	// TODO: Re-enable proto when proto rules are added
	resp := map[string]interface{}{
		"date":    id,
		"content": string(data),
	}

	// logger.Info(fmt.Sprintf("Returning news object with key: %s", key))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *NewsHandler) ListNews(w http.ResponseWriter, r *http.Request) {
	// TODO: Fix httplog v3 API usage  
	// logger := httplog.LogEntry(r.Context())
	ctx := r.Context()

	filenames, err := h.storage.ListNewsSummaries(ctx)
	if err != nil {
		// logger.Error("failed to list news summaries")
		http.Error(w, "failed to list news summaries", http.StatusInternalServerError)
		return
	}

	var summaries []map[string]interface{}
	for _, filename := range filenames {
		// Assuming format is "2025-05-07.md"
		date := strings.TrimSuffix(filename, ".md")
		summaries = append(summaries, map[string]interface{}{"date": date})
	}

	resp := map[string]interface{}{
		"summaries": summaries,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
