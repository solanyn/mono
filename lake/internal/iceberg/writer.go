package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	iceberggo "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	_ "github.com/apache/iceberg-go/catalog/rest"
	_ "github.com/apache/iceberg-go/io/gocloud"
	"github.com/apache/iceberg-go/table"
)

type Config struct {
	CatalogURI  string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Region    string
}

type Writer struct {
	cfg      Config
	mu       sync.Mutex
	catalogs map[string]catalog.Catalog
}

func NewWriter(cfg Config) *Writer {
	return &Writer{
		cfg:      cfg,
		catalogs: make(map[string]catalog.Catalog),
	}
}

func (w *Writer) getCatalog(ctx context.Context, warehouse string) (catalog.Catalog, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if cat, ok := w.catalogs[warehouse]; ok {
		return cat, nil
	}

	props := iceberggo.Properties{
		"type":                 "rest",
		"uri":                  w.cfg.CatalogURI,
		"warehouse":            warehouse,
		"s3.endpoint":          w.cfg.S3Endpoint,
		"s3.access-key-id":     w.cfg.S3AccessKey,
		"s3.secret-access-key": w.cfg.S3SecretKey,
		"s3.region":            w.cfg.S3Region,
		"s3.path-style-access": "true",
	}

	cat, err := catalog.Load(ctx, "lakekeeper-"+warehouse, props)
	if err != nil {
		return nil, fmt.Errorf("load catalog %s: %w", warehouse, err)
	}

	w.catalogs[warehouse] = cat
	return cat, nil
}

func (w *Writer) AppendBronze(ctx context.Context, tableName string, rows []map[string]interface{}, source, batchID string) error {
	return w.append(ctx, "bronze", tableName, rows, source, batchID)
}

func (w *Writer) AppendSilver(ctx context.Context, tableName string, rows []map[string]interface{}, source, batchID string) error {
	return w.append(ctx, "silver", tableName, rows, source, batchID)
}

func (w *Writer) AppendGold(ctx context.Context, tableName string, rows []map[string]interface{}, source, batchID string) error {
	return w.append(ctx, "gold", tableName, rows, source, batchID)
}

func (w *Writer) append(ctx context.Context, warehouse, tableName string, rows []map[string]interface{}, source, batchID string) error {
	if len(rows) == 0 {
		return nil
	}

	cat, err := w.getCatalog(ctx, warehouse)
	if err != nil {
		return err
	}

	tbl, err := cat.LoadTable(ctx, catalog.ToIdentifier("default", tableName))
	if err != nil {
		return fmt.Errorf("load table %s.default.%s: %w", warehouse, tableName, err)
	}

	ts := arrow.Timestamp(time.Now().UTC().UnixMicro())
	sources := make([]string, len(rows))
	ingestedAt := make([]arrow.Timestamp, len(rows))
	payloads := make([]string, len(rows))
	batchIDs := make([]string, len(rows))

	for i, row := range rows {
		sources[i] = source
		ingestedAt[i] = ts
		if v, ok := row["_raw_payload"]; ok && v != nil {
			if s, ok := v.(string); ok {
				payloads[i] = s
			}
		}
		if payloads[i] == "" {
			b, _ := json.Marshal(row)
			payloads[i] = string(b)
		}
		batchIDs[i] = batchID
	}

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "_source", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "_ingested_at", Type: &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}, Nullable: true},
		{Name: "_raw_payload", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "_batch_id", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)

	alloc := memory.NewGoAllocator()
	bldr := array.NewRecordBuilder(alloc, schema)
	defer bldr.Release()

	bldr.Field(0).(*array.StringBuilder).AppendValues(sources, nil)
	bldr.Field(1).(*array.TimestampBuilder).AppendValues(ingestedAt, nil)
	bldr.Field(2).(*array.StringBuilder).AppendValues(payloads, nil)
	bldr.Field(3).(*array.StringBuilder).AppendValues(batchIDs, nil)

	rec := bldr.NewRecord()
	defer rec.Release()

	arrowTbl := array.NewTableFromRecords(schema, []arrow.Record{rec})
	defer arrowTbl.Release()

	if _, err := tbl.AppendTable(ctx, arrowTbl, int64(len(rows)), nil); err != nil {
		return fmt.Errorf("append to %s.default.%s: %w", warehouse, tableName, err)
	}

	slog.Info("iceberg: appended", "rows", len(rows), "table", warehouse+".default."+tableName)
	return nil
}

func (w *Writer) ScanTable(ctx context.Context, warehouse, tableName string) (*table.Table, error) {
	cat, err := w.getCatalog(ctx, warehouse)
	if err != nil {
		return nil, err
	}

	tbl, err := cat.LoadTable(ctx, catalog.ToIdentifier("default", tableName))
	if err != nil {
		return nil, fmt.Errorf("load table %s.default.%s: %w", warehouse, tableName, err)
	}

	return tbl, nil
}
