package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/parquet-go/parquet-go"
)

type BronzeRow struct {
	Source     string `parquet:"_source"`
	IngestedAt int64  `parquet:"_ingested_at,timestamp(microsecond)"`
	RawPayload string `parquet:"_raw_payload"`
	BatchID    string `parquet:"_batch_id"`
}

func WriteBronze(rows []map[string]interface{}, source, batchID string) ([]byte, error) {
	now := time.Now().UTC().UnixMicro()
	var bronzeRows []BronzeRow
	for _, row := range rows {
		payload, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		bronzeRows = append(bronzeRows, BronzeRow{
			Source:     source,
			IngestedAt: now,
			RawPayload: string(payload),
			BatchID:    batchID,
		})
	}
	return writeParquet(bronzeRows)
}

func writeParquet[T any](rows []T) ([]byte, error) {
	var buf bytes.Buffer
	w := parquet.NewGenericWriter[T](&buf)
	if _, err := w.Write(rows); err != nil {
		return nil, fmt.Errorf("parquet write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("parquet close: %w", err)
	}
	return buf.Bytes(), nil
}

func ReadBronze(data []byte) ([]BronzeRow, error) {
	reader := bytes.NewReader(data)
	f, err := parquet.OpenFile(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("parquet open: %w", err)
	}

	r := parquet.NewGenericReader[BronzeRow](f)
	rows := make([]BronzeRow, f.NumRows())
	n, err := r.Read(rows)
	if err != nil {
		return nil, fmt.Errorf("parquet read: %w", err)
	}
	return rows[:n], nil
}
