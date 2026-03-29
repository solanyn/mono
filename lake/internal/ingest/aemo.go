package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const aemoURL = "https://visualisations.aemo.com.au/aemo/apps/api/report/ELEC_NEM_SUMMARY"

func IngestAEMO(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "aemo"

	rows, err := fetchAEMO(ctx)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch aemo: %w", err)
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "aemo.nem_summary", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "aemo", "nem_summary.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("aemo: wrote %d rows to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func fetchAEMO(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, aemoURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aemo http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		var arr []interface{}
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		return toMaps(arr), nil
	}

	summary, ok := data["ELEC_NEM_SUMMARY"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no ELEC_NEM_SUMMARY key")
	}
	return toMaps(summary), nil
}

func toMaps(arr []interface{}) []map[string]interface{} {
	var rows []map[string]interface{}
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			rows = append(rows, m)
		}
	}
	return rows
}
