package promote

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Result struct {
	Source   string
	Key      string
	RowCount int
}

var sourceConfig = map[string]string{
	"rba":    "f1-data.parquet",
	"abs":    "cpi_monthly.parquet",
	"aemo":   "nem_summary.parquet",
	"rss":    "news.parquet",
	"reddit": "ausfinance.parquet",
	"domain": "listings.parquet",
}

func PromoteSource(ctx context.Context, s3 *storage.Client, source, bronzeKey string) (Result, error) {
	start := time.Now()

	data, err := s3.GetObject(ctx, bronzeKey)
	if err != nil {
		return Result{}, fmt.Errorf("read bronze %s: %w", bronzeKey, err)
	}

	rows, err := storage.ReadBronze(data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues("promote_" + source).Inc()
		return Result{}, fmt.Errorf("parse bronze %s: %w", source, err)
	}

	silver, err := promoteGeneric(rows)
	if err != nil {
		metrics.IngestErrors.WithLabelValues("promote_" + source).Inc()
		return Result{}, fmt.Errorf("transform %s: %w", source, err)
	}

	dataset := silverName(source)
	key, err := s3.PutParquet(ctx, "silver", dataset, dataset+".parquet", silver)
	if err != nil {
		metrics.IngestErrors.WithLabelValues("promote_" + source).Inc()
		return Result{}, fmt.Errorf("write silver %s: %w", source, err)
	}

	metrics.PromoteTotal.WithLabelValues("silver").Inc()
	log.Printf("promote %s: %d rows → %s (%.1fs)", source, len(rows), key, time.Since(start).Seconds())
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func PromoteBronzeToSilver(ctx context.Context, s3 *storage.Client) error {
	for dataset, filename := range sourceConfig {
		data, err := s3.GetLatest(ctx, "bronze", dataset, filename)
		if err != nil {
			log.Printf("promote %s: no bronze data: %v", dataset, err)
			continue
		}

		rows, err := storage.ReadBronze(data)
		if err != nil {
			log.Printf("promote %s: read bronze: %v", dataset, err)
			continue
		}

		silver, err := promoteGeneric(rows)
		if err != nil {
			log.Printf("promote %s: transform: %v", dataset, err)
			continue
		}

		sd := silverName(dataset)
		key, err := s3.PutParquet(ctx, "silver", sd, sd+".parquet", silver)
		if err != nil {
			log.Printf("promote %s: write silver: %v", dataset, err)
			continue
		}

		metrics.PromoteTotal.WithLabelValues("silver").Inc()
		log.Printf("promote %s: %d rows → %s", dataset, len(rows), key)
	}
	return nil
}

func silverName(dataset string) string {
	switch dataset {
	case "rba":
		return "rba_rates"
	case "abs":
		return "abs_indicators"
	case "aemo":
		return "aemo_prices"
	case "rss":
		return "news_articles"
	case "reddit":
		return "reddit_sentiment"
	case "domain":
		return "domain_listings"
	default:
		return dataset
	}
}

func promoteRBA(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteABS(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteAEMO(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteRSS(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteReddit(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteDomain(rows []storage.BronzeRow) ([]byte, error) {
	return promoteGeneric(rows)
}

func promoteGeneric(rows []storage.BronzeRow) ([]byte, error) {
	var maps []map[string]interface{}
	for _, row := range rows {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(row.RawPayload), &m); err != nil {
			continue
		}
		maps = append(maps, m)
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("no rows to promote")
	}
	return storage.WriteBronze(maps, "promoted", "")
}
