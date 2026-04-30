package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const absBaURL = "https://api.data.abs.gov.au/data/ABS,BA_GCCSA?format=jsondata&detail=dataonly&startPeriod=2020-01"
const absMigrationURL = "https://api.data.abs.gov.au/data/ABS,ABS_NOM_VISA_FY?format=jsondata&detail=dataonly"

func IngestABSBuildingApprovals(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	return ingestSDMX(ctx, s3, bucket, "abs_ba", absBaURL, "building_approvals.parquet")
}

func IngestABSMigration(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	return ingestSDMX(ctx, s3, bucket, "abs_migration", absMigrationURL, "migration.parquet")
}

func ingestSDMX(ctx context.Context, s3 *storage.Client, bucket, source, url, filename string) (Result, error) {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch %s: %w", source, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("%s http %d", source, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, err
	}

	rows, err := parseABSJSON(body)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("parse %s: %w", source, err)
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, source, batchID)
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
	slog.Info("sdmx: wrote rows", "source", source, "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}
