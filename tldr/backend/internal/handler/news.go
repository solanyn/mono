package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type NewsResponse struct {
	ID       string `json:"id"`
	Markdown string `json:"markdown"`
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

	resp := NewsResponse{
		ID:       id,
		Markdown: string(data),
	}

	logger.Info(fmt.Sprintf("Returning news object with key: %s", key))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *NewsHandler) ListNews(w http.ResponseWriter, r *http.Request) {
	logger := httplog.LogEntry(r.Context())
	ctx := r.Context()

	summaries, err := h.storage.ListNewsSummaries(ctx)
	if err != nil {
		logger.Error("failed to list news summaries")
		http.Error(w, "failed to list news summaries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summaries)
}
