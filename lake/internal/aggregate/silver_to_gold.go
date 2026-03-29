package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

func SilverToGold(ctx context.Context, s3 *storage.Client, source, silverKey string) error {
	data, err := s3.GetObject(ctx, silverKey)
	if err != nil {
		return fmt.Errorf("read silver %s: %w", silverKey, err)
	}

	rows, err := storage.ReadBronze(data)
	if err != nil {
		return fmt.Errorf("parse silver %s: %w", source, err)
	}

	var maps []map[string]interface{}
	for _, row := range rows {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(row.RawPayload), &m); err != nil {
			continue
		}
		maps = append(maps, m)
	}

	if len(maps) == 0 {
		log.Printf("aggregate %s: no rows", source)
		return nil
	}

	gold, err := storage.WriteBronze(maps, "gold."+source, "")
	if err != nil {
		return fmt.Errorf("write gold %s: %w", source, err)
	}

	dataset := goldName(source)
	key, err := s3.PutParquet(ctx, "gold", dataset, dataset+".parquet", gold)
	if err != nil {
		return fmt.Errorf("put gold %s: %w", source, err)
	}

	metrics.PromoteTotal.WithLabelValues("gold").Inc()
	log.Printf("aggregate %s: %d rows → %s", source, len(maps), key)
	return nil
}

func goldName(source string) string {
	return source + "_agg"
}
