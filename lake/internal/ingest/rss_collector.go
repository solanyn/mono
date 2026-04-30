package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	readability "github.com/go-shiori/go-readability"
	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron/v3"
	"github.com/solanyn/mono/lake/internal/kafka"
	"github.com/solanyn/mono/lake/internal/metrics"
	"github.com/solanyn/mono/lake/internal/storage"
)

const (
	rssUserAgent      = "lake-rss/1.0 (+https://github.com/solanyn/mono)"
	maxConcurrency    = 4
	minDomainInterval = time.Second
	minExtractLen     = 200
	maxArticleBytes   = 2 * 1024 * 1024 // 2MB per article (was 10MB)
	seenTTL           = 24 * time.Hour
	seenMaxSize       = 50000
	startupStagger    = 5 * time.Second // per-feed stagger on startup
)

type ArticleEvent struct {
	Source        string       `json:"source"`
	FeedName      string       `json:"feed_name"`
	FeedSlug      string       `json:"feed_slug"`
	Category      FeedCategory `json:"category"`
	Item          ArticleItem  `json:"item"`
	RawFeedKey    string       `json:"raw_feed_key"`
	RawArticleKey string       `json:"raw_article_key"`
}

type ArticleItem struct {
	GUID          string `json:"guid"`
	Title         string `json:"title"`
	Link          string `json:"link"`
	Published     string `json:"published"`
	Author        string `json:"author"`
	RSSExcerpt    string `json:"rss_excerpt"`
	FullText      string `json:"full_text"`
	Partial       bool   `json:"partial"`
	DiscussionURL string `json:"discussion_url,omitempty"`
}

type RSSCollector struct {
	s3       *storage.Client
	producer *kafka.Producer
	bucket   string
	cron     *cron.Cron
	seen     *TTLCache
	sem      chan struct{}
	domains  map[string]time.Time
	domainMu sync.Mutex
	client   *http.Client
}

func NewRSSCollector(s3 *storage.Client, producer *kafka.Producer, bucket string) *RSSCollector {
	loc, _ := time.LoadLocation("Australia/Sydney")
	return &RSSCollector{
		s3:       s3,
		producer: producer,
		bucket:   bucket,
		cron:     cron.New(cron.WithLocation(loc)),
		seen:     NewTTLCache(seenTTL, seenMaxSize),
		sem:      make(chan struct{}, maxConcurrency),
		domains:  make(map[string]time.Time),
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (c *RSSCollector) Start() {
	for i, feed := range Feeds {
		f := feed
		jitter := time.Duration(rand.Intn(60)) * time.Second
		secs := int(f.Interval.Seconds())
		expr := fmt.Sprintf("@every %ds", secs)

		// Stagger startup over minutes to avoid burst of concurrent fetches
		delay := jitter + time.Duration(i)*startupStagger
		time.AfterFunc(delay, func() {
			c.processFeed(f)
			c.cron.AddFunc(expr, func() { c.processFeed(f) })
		})
	}
	c.cron.Start()
	slog.Info("rss_collector: started", "feeds", len(Feeds), "stagger", startupStagger.String())
}

func (c *RSSCollector) Stop() {
	c.cron.Stop()
	c.seen.Stop()
}

func (c *RSSCollector) processFeed(feed Feed) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	start := time.Now()
	source := "rss_feeds"

	rawXML, err := c.fetchURL(ctx, feed.URL)
	if err != nil {
		slog.Error("rss_collector: fetch feed", "feed", feed.Slug, "err", err)
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return
	}

	now := time.Now().UTC()
	feedKey := fmt.Sprintf("rss/%s/%s/%s/%d.xml",
		feed.Category, feed.Slug,
		now.Format("2006-01-02"), now.Unix())
	if err := c.s3.PutRaw(ctx, c.bucket, feedKey, "application/xml", rawXML); err != nil {
		slog.Error("rss_collector: store xml", "feed", feed.Slug, "err", err)
	}

	fp := gofeed.NewParser()
	parsed, err := fp.ParseString(string(rawXML))
	if err != nil {
		slog.Error("rss_collector: parse feed", "feed", feed.Slug, "err", err)
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return
	}

	var events []ArticleEvent
	var rows []map[string]interface{}

	for _, item := range parsed.Items {
		guid := itemGUID(item)
		dedupeKey := feed.Slug + "|" + guid

		if c.seen.Has(dedupeKey) {
			continue
		}
		c.seen.Set(dedupeKey)

		published := itemPublished(item)
		author := itemAuthor(item)
		excerpt := item.Description
		if excerpt == "" {
			excerpt = item.Content
		}

		var fullText, articleKey, discussionURL string
		partial := false
		articleLink := item.Link

		if feed.Category == CatYouTube {
			fullText = item.Description
			partial = fullText == ""
		} else if feed.Category == CatAggregator && (feed.Slug == "hacker-news" || feed.Slug == "lobsters") {
			discussionURL = item.Link
			externalURL := extractExternalURL(item)
			if externalURL != "" {
				articleLink = externalURL
				fullText, articleKey, partial = c.fetchAndExtract(ctx, feed, externalURL, guid, now)
			} else {
				partial = true
				fullText = excerpt
			}
		} else {
			fullText, articleKey, partial = c.fetchAndExtract(ctx, feed, item.Link, guid, now)
		}

		if partial && fullText == "" {
			fullText = excerpt
		}

		event := ArticleEvent{
			Source:        "rss",
			FeedName:      feed.Name,
			FeedSlug:      feed.Slug,
			Category:      feed.Category,
			RawFeedKey:    feedKey,
			RawArticleKey: articleKey,
			Item: ArticleItem{
				GUID:          guid,
				Title:         item.Title,
				Link:          articleLink,
				Published:     published,
				Author:        author,
				RSSExcerpt:    truncate(excerpt, 2000),
				FullText:      fullText,
				Partial:       partial,
				DiscussionURL: discussionURL,
			},
		}
		events = append(events, event)

		row, _ := json.Marshal(event)
		var m map[string]interface{}
		json.Unmarshal(row, &m)
		rows = append(rows, m)
	}

	if len(rows) == 0 {
		return
	}

	batchID := uuid.New().String()
	data, err := storage.WriteBronze(rows, "rss_feeds."+feed.Slug, batchID)
	if err != nil {
		slog.Error("rss_collector: write bronze", "feed", feed.Slug, "err", err)
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return
	}

	key, err := c.s3.PutParquet(ctx, c.bucket, "rss_feeds/"+feed.Slug, "articles.parquet", data)
	if err != nil {
		slog.Error("rss_collector: put s3", "feed", feed.Slug, "err", err)
		metrics.IngestErrors.WithLabelValues(source).Inc()
		return
	}

	if c.producer != nil {
		for _, event := range events {
			eventData, _ := json.Marshal(event)
			if err := c.producer.PublishRaw(ctx, "lake.articles.new", feed.Slug, eventData); err != nil {
				slog.Error("rss_collector: kafka publish", "feed", feed.Slug, "err", err)
			}
		}
	}

	metrics.IngestTotal.WithLabelValues(source).Inc()
	metrics.IngestDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
	metrics.LastIngestTimestamp.WithLabelValues(source).SetToCurrentTime()
	slog.Info("rss_collector: wrote articles", "feed", feed.Slug, "count", len(rows), "key", key)
}

func (c *RSSCollector) fetchAndExtract(ctx context.Context, feed Feed, articleURL, guid string, now time.Time) (fullText, articleKey string, partial bool) {
	if articleURL == "" {
		return "", "", true
	}

	html, err := c.fetchURL(ctx, articleURL)
	if err != nil {
		return "", "", true
	}

	guidHash := fmt.Sprintf("%x", sha256.Sum256([]byte(guid)))[:16]
	articleKey = fmt.Sprintf("articles/%s/%s/%s/%s.html",
		feed.Category, feed.Slug,
		now.Format("2006-01-02"), guidHash)
	if err := c.s3.PutRaw(ctx, c.bucket, articleKey, "text/html", html); err != nil {
		slog.Error("rss_collector: store html", "feed", feed.Slug, "err", err)
	}

	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return "", articleKey, true
	}

	article, err := readability.FromReader(strings.NewReader(string(html)), parsedURL)
	if err != nil || len(article.TextContent) < minExtractLen {
		return "", articleKey, true
	}

	return article.TextContent, articleKey, false
}

func (c *RSSCollector) fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	c.waitForDomain(rawURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", rssUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := time.ParseDuration(ra + "s"); err == nil {
				c.setDomainDelay(rawURL, secs)
			}
		}
		return nil, fmt.Errorf("rate limited: %s", rawURL)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArticleBytes))
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *RSSCollector) waitForDomain(rawURL string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	domain := parsed.Hostname()

	c.domainMu.Lock()
	last, ok := c.domains[domain]
	now := time.Now()
	if ok {
		wait := minDomainInterval - now.Sub(last)
		if wait > 0 {
			c.domainMu.Unlock()
			time.Sleep(wait)
			c.domainMu.Lock()
		}
	}
	c.domains[domain] = time.Now()
	c.domainMu.Unlock()
}

func (c *RSSCollector) setDomainDelay(rawURL string, delay time.Duration) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	c.domainMu.Lock()
	c.domains[parsed.Hostname()] = time.Now().Add(delay)
	c.domainMu.Unlock()
}

func itemGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

func itemPublished(item *gofeed.Item) string {
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC().Format(time.RFC3339)
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC().Format(time.RFC3339)
	}
	return ""
}

func itemAuthor(item *gofeed.Item) string {
	if item.Author != nil {
		return item.Author.Name
	}
	if len(item.Authors) > 0 {
		return item.Authors[0].Name
	}
	return ""
}

func extractExternalURL(item *gofeed.Item) string {
	if item.Link == "" {
		return ""
	}
	parsed, err := url.Parse(item.Link)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	if strings.Contains(host, "ycombinator.com") || strings.Contains(host, "lobste.rs") {
		for _, ext := range item.Extensions {
			for _, exts := range ext {
				for _, e := range exts {
					if e.Name == "url" {
						return e.Value
					}
				}
			}
		}
		if item.Content != "" {
			return ""
		}
		return ""
	}
	return item.Link
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
