package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

func IngestNSWPropertyLicences(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	return ingestNSWLicences(ctx, s3, bucket, "nsw_property_licences",
		os.Getenv("API_NSW_PROPERTY_API_KEY"), os.Getenv("API_NSW_PROPERTY_API_SECRET"),
		"/propertyregister/v1/browse", "property_licences.parquet")
}

func IngestNSWTradesLicences(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	return ingestNSWLicences(ctx, s3, bucket, "nsw_trades_licences",
		os.Getenv("API_NSW_TRADES_API_KEY"), os.Getenv("API_NSW_TRADES_API_SECRET"),
		"/tradesregister/v1/browse", "trades_licences.parquet")
}

func ingestNSWLicences(ctx context.Context, s3 *storage.Client, bucket, source, apiKey, apiSecret, endpoint, filename string) (Result, error) {
	start := time.Now()

	if apiKey == "" || apiSecret == "" {
		log.Printf("%s: API key/secret not set, skipping", source)
		return Result{}, nil
	}

	token, err := nswGovToken(ctx, apiKey, apiSecret)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("%s auth: %w", source, err)
	}

	var allRows []map[string]interface{}
	for page := 1; page <= 50; page++ {
		path := fmt.Sprintf("%s?searchText=&pageNum=%d&pageSize=100", endpoint, page)
		body, err := nswGovGet(ctx, path, token, apiKey, nil)
		if err != nil {
			log.Printf("%s: page %d: %v", source, page, err)
			break
		}

		var rows []map[string]interface{}
		if err := json.Unmarshal(body, &rows); err != nil {
			log.Printf("%s: page %d parse: %v", source, page, err)
			break
		}
		if len(rows) == 0 {
			break
		}
		allRows = append(allRows, rows...)
		time.Sleep(200 * time.Millisecond)
	}

	if len(allRows) == 0 {
		log.Printf("%s: no records fetched", source)
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(allRows, "nsw."+source, batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, source, filename, data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("%s: wrote %d records to %s", source, len(allRows), key)
	return Result{Source: source, Key: key, RowCount: len(allRows)}, nil
}
