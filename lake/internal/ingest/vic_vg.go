package ingest

import (
	"context"
	"encoding/json"
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

const (
	vicVGCKANBase = "https://discover.data.vic.gov.au/api/3/action/package_show"
)

// DataVic CKAN datasets for VIC property sales
var vicVGDatasets = []struct {
	ID       string
	Category string
}{
	{"victorian-property-sales-report-median-house-by-suburb", "house"},
	{"victorian-property-sales-report-median-unit-by-suburb", "unit"},
	{"victorian-property-sales-report-time-series", "time_series"},
}

type ckanResponse struct {
	Success bool       `json:"success"`
	Result  ckanResult `json:"result"`
}

type ckanResult struct {
	Name      string         `json:"name"`
	Title     string         `json:"title"`
	Resources []ckanResource `json:"resources"`
}

type ckanResource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Format      string `json:"format"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	ReleaseDate string `json:"release_date"`
}

func IngestVicVG(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "vic_vg"

	client := &http.Client{Timeout: 60 * time.Second}
	var allRows []map[string]interface{}

	for _, ds := range vicVGDatasets {
		resources, err := fetchCKANResources(ctx, client, ds.ID)
		if err != nil {
			log.Printf("vic_vg: ckan %s: %v", ds.ID, err)
			continue
		}

		for _, res := range resources {
			// Download the XLS file
			xlsData, err := fetchURL(ctx, client, res.URL)
			if err != nil {
				log.Printf("vic_vg: fetch %s (%s): %v", res.Name, ds.Category, err)
				continue
			}

			// Store raw XLS in bronze
			now := time.Now().UTC()
			rawKey := fmt.Sprintf("vic_vg/%s/%s/%s.xls",
				ds.Category, now.Format("2006-01-02"), sanitizeFilename(res.Name))
			if err := s3.PutRaw(ctx, bucket, rawKey, "application/vnd.ms-excel", xlsData); err != nil {
				log.Printf("vic_vg: store raw %s: %v", res.Name, err)
			}

			row := map[string]interface{}{
				"dataset":      ds.ID,
				"category":     ds.Category,
				"resource_id":  res.ID,
				"resource_name": res.Name,
				"url":          res.URL,
				"format":       res.Format,
				"period_start": res.PeriodStart,
				"period_end":   res.PeriodEnd,
				"release_date": res.ReleaseDate,
				"raw_key":      rawKey,
				"file_size":    len(xlsData),
			}
			allRows = append(allRows, row)
			log.Printf("vic_vg: stored %s/%s (%d bytes)", ds.Category, res.Name, len(xlsData))
		}
	}

	if len(allRows) == 0 {
		log.Println("vic_vg: no data fetched")
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(allRows, "vic_vg.property_sales", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "vic_vg", "property_sales.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("vic_vg: wrote %d resources to %s", len(allRows), key)
	return Result{Source: source, Key: key, RowCount: len(allRows)}, nil
}

func fetchCKANResources(ctx context.Context, client *http.Client, datasetID string) ([]ckanResource, error) {
	url := fmt.Sprintf("%s?id=%s", vicVGCKANBase, datasetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ckan http %d for %s", resp.StatusCode, datasetID)
	}

	var ckan ckanResponse
	if err := json.NewDecoder(resp.Body).Decode(&ckan); err != nil {
		return nil, fmt.Errorf("decode ckan: %w", err)
	}

	if !ckan.Success {
		return nil, fmt.Errorf("ckan api failed for %s", datasetID)
	}

	return ckan.Result.Resources, nil
}

func fetchURL(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lake-ingest/1.0 (+https://github.com/solanyn/mono)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024)) // 50MB limit for XLS
	if err != nil {
		return nil, err
	}
	return body, nil
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	return strings.ToLower(r.Replace(s))
}


