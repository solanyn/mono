package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

var rssFeeds = map[string]string{
	"guardian_au_business": "https://www.theguardian.com/au/business/rss",
	"rba_media_releases":   "https://www.rba.gov.au/rss/rss-cb-media-releases.xml",
	"rba_speeches":         "https://www.rba.gov.au/rss/rss-cb-speeches.xml",
}

func IngestRSS(ctx context.Context, s3 *storage.Client) error {
	start := time.Now()
	source := "rss"

	fp := gofeed.NewParser()
	var rows []map[string]interface{}

	for sourceID, url := range rssFeeds {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			log.Printf("rss: skip %s: %v", sourceID, err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("rss: skip %s: %v", sourceID, err)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		feed, err := fp.ParseString(string(body))
		if err != nil {
			log.Printf("rss: parse %s: %v", sourceID, err)
			continue
		}

		for _, item := range feed.Items {
			published := ""
			if item.PublishedParsed != nil {
				published = item.PublishedParsed.UTC().Format(time.RFC3339)
			} else if item.UpdatedParsed != nil {
				published = item.UpdatedParsed.UTC().Format(time.RFC3339)
			}
			rows = append(rows, map[string]interface{}{
				"title":        item.Title,
				"url":          item.Link,
				"published_at": published,
				"source":       sourceID,
				"summary":      item.Description,
			})
		}
	}

	if len(rows) == 0 {
		log.Println("rss: no articles fetched")
		return nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "rss", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, "bronze", "rss", "news.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("rss: wrote %d articles to %s", len(rows), key)
	return nil
}

func init() {
	_ = json.Marshal
}
