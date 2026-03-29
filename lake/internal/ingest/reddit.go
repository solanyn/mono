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
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

var redditFeeds = map[string]string{
	"hot": "https://www.reddit.com/r/AusFinance/hot.json?limit=25",
	"new": "https://www.reddit.com/r/AusFinance/new.json?limit=25",
}

const redditUserAgent = "lake-ingest/0.1 (Australian macro-financial data lake)"

func IngestReddit(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "reddit"

	seen := make(map[string]bool)
	var rows []map[string]interface{}

	for _, url := range redditFeeds {
		posts, err := fetchReddit(ctx, url)
		if err != nil {
			log.Printf("reddit: %v", err)
			continue
		}
		for _, p := range posts {
			id, _ := p["post_id"].(string)
			if seen[id] {
				continue
			}
			seen[id] = true
			rows = append(rows, p)
		}
	}

	if len(rows) == 0 {
		log.Println("reddit: no posts fetched")
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "reddit.ausfinance", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}

	key, err := s3.PutParquet(ctx, bucket, "reddit", "ausfinance.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	log.Printf("reddit: wrote %d posts to %s", len(rows), key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func fetchReddit(ctx context.Context, url string) ([]map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", redditUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listing struct {
		Data struct {
			Children []struct {
				Data map[string]interface{} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var posts []map[string]interface{}
	for _, child := range listing.Data.Children {
		d := child.Data
		flair, _ := d["link_flair_text"].(string)
		score, _ := d["score"].(float64)
		numComments, _ := d["num_comments"].(float64)
		posts = append(posts, map[string]interface{}{
			"post_id":      d["id"],
			"title":        d["title"],
			"url":          d["url"],
			"score":        int(score),
			"num_comments": int(numComments),
			"flair":        flair,
			"created_utc":  d["created_utc"],
		})
	}
	return posts, nil
}
