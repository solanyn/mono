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

const rbaCreditURL = "https://rba.gov.au/statistics/tables/csv/d1-data.csv"

func IngestRBACredit(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "rba_credit"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rbaCreditURL, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch rba credit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("rba credit http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, err
	}

	rows, err := parseRBACreditCSV(string(body))
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("parse rba credit: %w", err)
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "rba.d1", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "rba_credit", "d1-data.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("rba_credit: wrote %d rows to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func parseRBACreditCSV(raw string) ([]map[string]interface{}, error) {
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
