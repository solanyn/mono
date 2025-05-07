package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	newspb "tldr/gen/proto"
	"tldr/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v2"
)

type NewsHandler struct {
	storage *storage.Client
}

func NewNewsHandler(s *storage.Client) *NewsHandler {
	return &NewsHandler{storage: s}
}

func (h *NewsHandler) GetNews(w http.ResponseWriter, r *http.Request) {
	logger := httplog.LogEntry(r.Context())
	id := chi.URLParam(r, "id")
	key := fmt.Sprintf("news/%s.md", id)

	logger.Info(fmt.Sprintf("Fetching news object with key: %s", key))
	obj, err := h.storage.GetObject(context.Background(), key)
	if err != nil {
		http.Error(w, `{"error": "Not found"}`, http.StatusNotFound)
		return
	}
	defer obj.Close()

	logger.Info(fmt.Sprintf("Reading news object with key: %s", key))
	data, err := io.ReadAll(obj)
	if err != nil {
		http.Error(w, `{"error": "Failed to read file"}`, http.StatusInternalServerError)
		return
	}

	resp := &newspb.GetNewsSummaryResponse{
		Date:    id,
		Content: string(data),
	}

	logger.Info(fmt.Sprintf("Returning news object with key: %s", key))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *NewsHandler) ListNews(w http.ResponseWriter, r *http.Request) {
	logger := httplog.LogEntry(r.Context())
	ctx := r.Context()

	filenames, err := h.storage.ListNewsSummaries(ctx)
	if err != nil {
		logger.Error("failed to list news summaries")
		http.Error(w, "failed to list news summaries", http.StatusInternalServerError)
		return
	}

	var summaries []*newspb.NewsSummary
	for _, filename := range filenames {
		// Assuming format is "2025-05-07.md"
		date := strings.TrimSuffix(filename, ".md")
		summaries = append(summaries, &newspb.NewsSummary{Date: date})
	}

	resp := &newspb.ListNewsSummariesResponse{
		Summaries: summaries,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
