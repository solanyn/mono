package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const sqmBaseURL = "https://sqmresearch.com.au/property"

// SQM metrics to scrape per suburb
var sqmMetrics = []struct {
	Slug     string
	Path     string
	TypeParam string
}{
	{"vacancy-rates", "vacancy-rates", "c"},
	{"asking-prices-houses", "asking-property-prices", "c"},
	{"asking-prices-units", "asking-property-prices", "u"},
	{"stock-on-market", "stock-on-market", "c"},
	{"asking-rent-houses", "weekly-rents", "c"},
	{"asking-rent-units", "weekly-rents", "u"},
}

// Target Melbourne suburbs for property research
var melbourneSuburbs = []struct {
	Name     string
	Region   string // SQM region param
	Postcode string
}{
	// Eastern suburbs (buying targets)
	{"Blackburn", "vic-Blackburn", "3130"},
	{"Blackburn South", "vic-Blackburn+South", "3130"},
	{"Mitcham", "vic-Mitcham", "3132"},
	{"Nunawading", "vic-Nunawading", "3131"},
	{"Vermont South", "vic-Vermont+South", "3133"},
	{"Ringwood", "vic-Ringwood", "3134"},
	{"Glen Waverley", "vic-Glen+Waverley", "3150"},
	{"Doncaster", "vic-Doncaster", "3108"},
	{"Box Hill", "vic-Box+Hill", "3128"},
	{"Camberwell", "vic-Camberwell", "3124"},
	// Interesting rental suburbs
	{"Fitzroy", "vic-Fitzroy", "3065"},
	{"Northcote", "vic-Northcote", "3070"},
	{"Thornbury", "vic-Thornbury", "3071"},
	{"Brunswick", "vic-Brunswick", "3056"},
	{"Footscray", "vic-Footscray", "3011"},
	{"St Kilda", "vic-St+Kilda", "3182"},
	{"Richmond", "vic-Richmond", "3121"},
	{"Yarraville", "vic-Yarraville", "3013"},
	// Metro aggregate
	{"Melbourne", "vic-Melbourne", ""},
}

// chartDataRe extracts JSON arrays from SQM chart JavaScript
var chartDataRe = regexp.MustCompile(`data:\s*\[([^\]]+)\]`)
var chartLabelsRe = regexp.MustCompile(`labels:\s*\[([^\]]+)\]`)

func IngestSQM(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "sqm_research"

	client := &http.Client{Timeout: 30 * time.Second}
	var allRows []map[string]interface{}

	for _, suburb := range melbourneSuburbs {
		for _, metric := range sqmMetrics {
			url := fmt.Sprintf("%s/%s?region=%s&type=%s&t=1",
				sqmBaseURL, metric.Path, suburb.Region, metric.TypeParam)

			html, err := fetchSQMPage(ctx, client, url)
			if err != nil {
				slog.Error("sqm: fetch", "suburb", suburb.Name, "metric", metric.Slug, "err", err)
				continue
			}

			// Store raw HTML
			now := time.Now().UTC()
			rawKey := fmt.Sprintf("sqm/%s/%s/%s/%s.html",
				metric.Slug, suburb.Name,
				now.Format("2006-01-02"), now.Format("150405"))
			if err := s3.PutRaw(ctx, bucket, rawKey, "text/html", html); err != nil {
				slog.Error("sqm: store", "suburb", suburb.Name, "metric", metric.Slug, "err", err)
			}

			// Extract chart data if present
			chartData := extractChartData(string(html))

			row := map[string]interface{}{
				"suburb":     suburb.Name,
				"postcode":   suburb.Postcode,
				"region":     suburb.Region,
				"metric":     metric.Slug,
				"url":        url,
				"raw_key":    rawKey,
				"chart_data": chartData,
				"fetched_at": now.Format(time.RFC3339),
			}
			allRows = append(allRows, row)

			// Rate limit: 1 request per second to be polite
			time.Sleep(time.Second)
		}
	}

	if len(allRows) == 0 {
		slog.Info("sqm: no data fetched")
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(allRows, "sqm_research.melbourne", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "sqm_research", "melbourne.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("sqm: wrote suburb-metrics", "count", len(allRows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(allRows)}, nil
}

func fetchSQMPage(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// SQM blocks bare curl; mimic browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")
	req.Header.Set("Referer", "https://sqmresearch.com.au/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func extractChartData(html string) string {
	var parts []string

	labels := chartLabelsRe.FindAllStringSubmatch(html, -1)
	for i, m := range labels {
		if len(m) > 1 {
			parts = append(parts, fmt.Sprintf("labels_%d:[%s]", i, strings.TrimSpace(m[1])))
		}
	}

	data := chartDataRe.FindAllStringSubmatch(html, -1)
	for i, m := range data {
		if len(m) > 1 {
			parts = append(parts, fmt.Sprintf("data_%d:[%s]", i, strings.TrimSpace(m[1])))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ")
}
