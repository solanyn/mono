package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

var asxTickers = []string{"BHP", "CBA", "CSL", "NAB", "WBC", "ANZ", "WES", "MQG", "FMG", "RIO"}

func IngestASX(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "asx"
	var rows []map[string]interface{}

	for _, ticker := range asxTickers {
		url := fmt.Sprintf("https://asx.api.markitdigital.com/asx-research/1.0/companies/%s/announcements?count=20&market_sensitive=false", ticker)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Error("asx: fetch", "ticker", ticker, "err", err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			continue
		}
		items, _ := data["data"].([]interface{})
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				m["ticker"] = ticker
				rows = append(rows, m)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "asx.announcements", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "asx", "announcements.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("asx: wrote announcements", "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}
