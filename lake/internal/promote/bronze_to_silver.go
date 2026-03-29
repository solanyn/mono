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

func PromoteBronzeToSilver(ctx context.Context, s3 *storage.Client) error {
	sources := []struct {
		dataset  string
		filename string
		promote  func([]storage.BronzeRow) ([]byte, error)
	}{
		{"rba", "f1-data.parquet", promoteRBA},
		{"abs", "cpi_monthly.parquet", promoteABS},
		{"aemo", "nem_summary.parquet", promoteAEMO},
		{"rss", "news.parquet", promoteRSS},
		{"reddit", "ausfinance.parquet", promoteReddit},
		{"domain", "listings.parquet", promoteDomain},
	}

	for _, src := range sources {
		start := time.Now()
		data, err := s3.GetLatest(ctx, "bronze", src.dataset, src.filename)
		if err != nil {
			log.Printf("promote %s: no bronze data: %v", src.dataset, err)
			continue
		}

		rows, err := storage.ReadBronze(data)
		if err != nil {
			log.Printf("promote %s: read bronze: %v", src.dataset, err)
			metrics.IngestErrors.WithLabelValues("promote_" + src.dataset).Inc()
			continue
		}

		silver, err := src.promote(rows)
		if err != nil {
			log.Printf("promote %s: transform: %v", src.dataset, err)
			metrics.IngestErrors.WithLabelValues("promote_" + src.dataset).Inc()
			continue
		}

		silverDataset := silverName(src.dataset)
		key, err := s3.PutParquet(ctx, "silver", silverDataset, silverDataset+".parquet", silver)
		if err != nil {
			log.Printf("promote %s: write silver: %v", src.dataset, err)
			metrics.IngestErrors.WithLabelValues("promote_" + src.dataset).Inc()
			continue
		}

		metrics.PromoteTotal.WithLabelValues("silver").Inc()
		log.Printf("promote %s: %d rows → %s (%.1fs)", src.dataset, len(rows), key, time.Since(start).Seconds())
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
