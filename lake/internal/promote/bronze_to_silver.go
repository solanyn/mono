package promote

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	icebergw "github.com/solanyn/mono/lake/internal/iceberg"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

type Result struct {
	Source   string
	Table    string
	RowCount int
}

func PromoteSource(ctx context.Context, s3 *storage.Client, iceWriter *icebergw.Writer, bronzeBucket, source, bronzeKey string) (Result, error) {
	if iceWriter == nil {
		return Result{}, fmt.Errorf("promote %s: iceberg writer required", source)
	}

	start := time.Now()

	data, err := s3.GetObject(ctx, bronzeBucket, bronzeKey)
	if err != nil {
		return Result{}, fmt.Errorf("read bronze %s: %w", bronzeKey, err)
	}

	rows, err := storage.ReadBronze(data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues("promote_" + source).Inc()
		return Result{}, fmt.Errorf("parse bronze %s: %w", source, err)
	}

	if len(rows) == 0 {
		slog.Info("promote: no rows", "source", source)
		return Result{Source: source, Table: silverName(source), RowCount: 0}, nil
	}

	maps := bronzeRowsToMaps(rows)
	table := silverName(source)

	if err := iceWriter.AppendSilver(ctx, table, maps, source, bronzeKey); err != nil {
		metrics.IngestErrors.WithLabelValues("promote_" + source).Inc()
		return Result{}, fmt.Errorf("iceberg append silver %s: %w", table, err)
	}

	metrics.PromoteTotal.WithLabelValues("silver").Inc()
	slog.Info("promote: appended", "source", source, "rows", len(rows), "table", "silver."+table, "dur_sec", time.Since(start).Seconds())
	return Result{Source: source, Table: table, RowCount: len(rows)}, nil
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
	case "nsw_vg":
		return "nsw_vg_sales"
	default:
		return dataset
	}
}

func bronzeRowsToMaps(rows []storage.BronzeRow) []map[string]interface{} {
	maps := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(row.RawPayload), &m); err != nil {
			continue
		}
		maps = append(maps, m)
	}
	return maps
}
