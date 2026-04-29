package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

func IngestNSWFuel(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "nsw_fuel"

	apiKey := os.Getenv("API_NSW_FUEL_API_KEY")
	apiSecret := os.Getenv("API_NSW_FUEL_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		slog.Info("nsw_fuel: API_NSW_FUEL_API_KEY/SECRET not set, skipping")
		return Result{}, nil
	}

	token, err := nswGovToken(ctx, apiKey, apiSecret)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("nsw fuel auth: %w", err)
	}

	body, err := nswGovGet(ctx, "/FuelPriceCheck/v2/fuel/prices", token, apiKey, fuelHeaders())
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("nsw fuel fetch: %w", err)
	}

	var resp struct {
		Prices []map[string]interface{} `json:"prices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("nsw fuel parse: %w", err)
	}

	if len(resp.Prices) == 0 {
		slog.Info("nsw_fuel: no prices returned")
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(resp.Prices, "nsw.fuel_prices", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "nsw_fuel", "prices.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("nsw_fuel: wrote prices", "count", len(resp.Prices), "key", key)
	return Result{Source: source, Key: key, RowCount: len(resp.Prices)}, nil
}
