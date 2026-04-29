package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

func IngestGitHubTrending(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "github_trending"

	url := "https://api.github.com/search/repositories?q=stars:>100+pushed:>2026-03-01&sort=stars&order=desc&per_page=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch github: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("github http %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("parse github: %w", err)
	}

	rows := make([]map[string]interface{}, 0, len(result.Items))
	for _, item := range result.Items {
		rows = append(rows, map[string]interface{}{
			"full_name":   item["full_name"],
			"description": item["description"],
			"language":    item["language"],
			"stars":       item["stargazers_count"],
			"forks":       item["forks_count"],
			"open_issues": item["open_issues_count"],
			"url":         item["html_url"],
			"created_at":  item["created_at"],
			"pushed_at":   item["pushed_at"],
		})
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "github.trending", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "github_trending", "repos.parquet", data)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("github_trending: wrote repos", "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

var trackedPyPIPackages = []string{"pandas", "numpy", "polars", "duckdb", "pyarrow", "fastapi", "pydantic", "httpx", "uv", "ruff"}

func IngestPyPIStats(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "pypi_stats"
	var rows []map[string]interface{}

	for _, pkg := range trackedPyPIPackages {
		url := fmt.Sprintf("https://pypistats.org/api/packages/%s/recent", pkg)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			continue
		}
		dataMap, _ := data["data"].(map[string]interface{})
		if dataMap != nil {
			dataMap["package"] = pkg
			rows = append(rows, dataMap)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	d, err := storage.WriteBronze(rows, "pypi.stats", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "pypi_stats", "recent.parquet", d)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("pypi_stats: wrote packages", "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

var trackedNpmPackages = []string{"react", "next", "vue", "svelte", "typescript", "esbuild", "vite", "bun", "deno", "hono"}

func IngestNpmStats(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "npm_stats"
	var rows []map[string]interface{}

	for _, pkg := range trackedNpmPackages {
		url := fmt.Sprintf("https://api.npmjs.org/downloads/point/last-week/%s", pkg)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			continue
		}
		rows = append(rows, data)
		time.Sleep(100 * time.Millisecond)
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	d, err := storage.WriteBronze(rows, "npm.stats", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "npm_stats", "weekly.parquet", d)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("npm_stats: wrote packages", "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}

func IngestHN(ctx context.Context, s3 *storage.Client, bucket string) (Result, error) {
	start := time.Now()
	source := "hn_stories"

	url := "https://hn.algolia.com/api/v1/search_by_date?tags=story&hitsPerPage=50"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("fetch hn: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("hn http %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Hits []map[string]interface{} `json:"hits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("parse hn: %w", err)
	}

	rows := make([]map[string]interface{}, 0, len(result.Hits))
	for _, hit := range result.Hits {
		rows = append(rows, map[string]interface{}{
			"title":      hit["title"],
			"url":        hit["url"],
			"author":     hit["author"],
			"points":     hit["points"],
			"comments":   hit["num_comments"],
			"story_id":   hit["objectID"],
			"created_at": hit["created_at"],
		})
	}

	if len(rows) == 0 {
		return Result{}, nil
	}

	batchID := uuid.New().String()
	d, err := storage.WriteBronze(rows, "hn.stories", batchID)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("write bronze: %w", err)
	}
	key, err := s3.PutParquet(ctx, bucket, "hn_stories", "recent.parquet", d)
	if err != nil {
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return Result{}, fmt.Errorf("put s3: %w", err)
	}
	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("hn_stories: wrote stories", "count", len(rows), "key", key)
	return Result{Source: source, Key: key, RowCount: len(rows)}, nil
}
