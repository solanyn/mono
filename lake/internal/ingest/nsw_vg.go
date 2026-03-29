package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const nswVGBaseURL = "https://valuation.property.nsw.gov.au/embed/propertySalesInformation"

var nswVGHeaders = []string{
	"district_code", "property_id", "sale_counter", "download_date",
	"property_name", "property_unit_number", "property_house_number",
	"property_street_name", "property_locality", "property_post_code",
	"area", "area_type", "contract_date", "settlement_date",
	"purchase_price", "zoning", "nature_of_property", "primary_purpose",
	"strata_lot_number", "component_code", "sale_code",
	"percent_interest", "dealing_number",
}

func IngestNSWVG(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "nsw_vg"

	currentYear := time.Now().Year()
	var allRows []map[string]interface{}

	for year := currentYear - 1; year <= currentYear; year++ {
		rows, err := fetchNSWVGYear(ctx, year)
		if err != nil {
			log.Printf("nsw_vg: year %d: %v", year, err)
			continue
		}
		allRows = append(allRows, rows...)
		log.Printf("nsw_vg: fetched %d rows for %d", len(rows), year)
	}

	if len(allRows) == 0 {
		log.Println("nsw_vg: no data fetched")
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(allRows, "nsw_vg.bulk_sales", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "nsw_vg", "bulk_sales.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("nsw_vg: wrote %d rows to %s", len(allRows), key)
	return Result{Source: source, Key: key, RowCount: len(allRows)}, nil
}

func fetchNSWVGYear(ctx context.Context, year int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s?year=%d", nswVGBaseURL, year)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nsw_vg http %d for year %d", resp.StatusCode, year)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "zip") || strings.HasPrefix(string(body[:4]), "PK") {
		return parseNSWVGZip(body, year)
	}
	return parseNSWVGCSV(bytes.NewReader(body), year)
}

func parseNSWVGZip(data []byte, year int) ([]map[string]interface{}, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var allRows []map[string]interface{}
	for _, f := range reader.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			log.Printf("nsw_vg: skip %s: %v", f.Name, err)
			continue
		}
		rows, err := parseNSWVGCSV(rc, year)
		rc.Close()
		if err != nil {
			log.Printf("nsw_vg: parse %s: %v", f.Name, err)
			continue
		}
		allRows = append(allRows, rows...)
	}
	return allRows, nil
}

func parseNSWVGCSV(r io.Reader, year int) ([]map[string]interface{}, error) {
	csvReader := csv.NewReader(r)
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1

	var rows []map[string]interface{}
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(record) < 2 {
			continue
		}
		if record[0] == "A" || record[0] == "district_code" {
			continue
		}

		row := make(map[string]interface{})
		row["year"] = year
		for i, header := range nswVGHeaders {
			if i < len(record) {
				row[header] = strings.TrimSpace(record[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
