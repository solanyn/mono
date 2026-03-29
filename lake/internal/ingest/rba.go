package ingest

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const rbaURL = "https://rba.gov.au/statistics/tables/csv/f1-data.csv"

func IngestRBA(ctx context.Context, s3 *storage.Client) (Result, error) {
	start := time.Now()
	source := "rba"

	rows, err := fetchRBA(ctx)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch rba: %w", err)
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "rba.f1", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, "bronze", "rba", "f1-data.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("rba: wrote %d rows to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func fetchRBA(ctx context.Context) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rbaURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rba http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseRBACSV(string(body))
}

func parseRBACSV(raw string) ([]map[string]interface{}, error) {
	lines := strings.Split(raw, "\n")
	headerIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "Series ID") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("could not find header row")
	}

	r := csv.NewReader(strings.NewReader(strings.Join(lines[headerIdx:], "\n")))
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}

	var rows []map[string]interface{}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		row := make(map[string]interface{})
		for i, h := range headers {
			if i < len(record) {
				row[h] = record[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
