package aggregate

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

func SilverToGold(ctx context.Context, s3 *storage.Client, iceWriter *icebergw.Writer, bronzeBucket, source, bronzeKey string) (Result, error) {
	if iceWriter == nil {
		return Result{}, fmt.Errorf("aggregate %s: iceberg writer required", source)
	}

	start := time.Now()

	data, err := s3.GetObject(ctx, bronzeBucket, bronzeKey)
	if err != nil {
		return Result{}, fmt.Errorf("read bronze %s: %w", bronzeKey, err)
	}

	rows, err := storage.ReadBronze(data)
	if err != nil {
		return Result{}, fmt.Errorf("parse bronze %s: %w", source, err)
	}

	maps := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(row.RawPayload), &m); err != nil {
			continue
		}
		maps = append(maps, m)
	}

	table := goldName(source)
	if len(maps) == 0 {
		slog.Info("aggregate: no rows", "source", source)
		return Result{Source: source, Table: table, RowCount: 0}, nil
	}

	if err := iceWriter.AppendGold(ctx, table, maps, source, bronzeKey); err != nil {
		return Result{}, fmt.Errorf("iceberg append gold %s: %w", table, err)
	}

	metrics.PromoteTotal.WithLabelValues("gold").Inc()
	slog.Info("aggregate: appended", "source", source, "rows", len(maps), "table", "gold."+table, "dur_sec", time.Since(start).Seconds())
	return Result{Source: source, Table: table, RowCount: len(maps)}, nil
}

func goldName(source string) string {
	return source + "_agg"
}
