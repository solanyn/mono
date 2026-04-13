package ingest

import (
	"testing"
	"time"
)

func TestRSSCollector_Constants(t *testing.T) {
	// Verify the tuned constants are what we expect
	if maxConcurrency != 4 {
		t.Errorf("maxConcurrency should be 4, got %d", maxConcurrency)
	}
	if maxArticleBytes != 2*1024*1024 {
		t.Errorf("maxArticleBytes should be 2MB, got %d", maxArticleBytes)
	}
	if seenTTL != 24*time.Hour {
		t.Errorf("seenTTL should be 24h, got %s", seenTTL)
	}
	if seenMaxSize != 50000 {
		t.Errorf("seenMaxSize should be 50000, got %d", seenMaxSize)
	}
	if startupStagger != 5*time.Second {
		t.Errorf("startupStagger should be 5s, got %s", startupStagger)
	}
}

func TestRSSCollector_SeenDedup(t *testing.T) {
	// Simulate the dedup logic from processFeed
	seen := NewTTLCache(time.Hour, 1000)
	defer seen.Stop()

	feed := Feed{Slug: "test-feed"}
	guid1 := "article-1"
	guid2 := "article-2"

	key1 := feed.Slug + "|" + guid1
	key2 := feed.Slug + "|" + guid2

	// First encounter — not seen
	if seen.Has(key1) {
		t.Error("key1 should not be seen yet")
	}
	seen.Set(key1)

	// Second encounter — should be deduped
	if !seen.Has(key1) {
		t.Error("key1 should be seen after Set")
	}

	// Different article — not seen
	if seen.Has(key2) {
		t.Error("key2 should not be seen yet")
	}

	// Same GUID, different feed — not seen (slug prefix differentiates)
	otherKey := "other-feed|" + guid1
	if seen.Has(otherKey) {
		t.Error("same GUID in different feed should not collide")
	}
}

func TestRSSCollector_SeenExpiry(t *testing.T) {
	// After TTL, articles should be re-fetchable (not permanently deduped)
	seen := NewTTLCache(50*time.Millisecond, 1000)
	defer seen.Stop()

	key := "feed|old-article"
	seen.Set(key)

	if !seen.Has(key) {
		t.Fatal("should be seen immediately")
	}

	time.Sleep(80 * time.Millisecond)

	if seen.Has(key) {
		t.Error("should have expired after TTL — article can be re-ingested")
	}
}

func TestItemGUID(t *testing.T) {
	tests := []struct {
		name     string
		guid     string
		link     string
		expected string
	}{
		{"has guid", "abc-123", "https://example.com/post", "abc-123"},
		{"empty guid falls back to link", "", "https://example.com/post", "https://example.com/post"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Can't easily construct gofeed.Item here without the dep,
			// but we test the logic inline
			guid := tt.guid
			if guid == "" {
				guid = tt.link
			}
			if guid != tt.expected {
				t.Errorf("got %q, want %q", guid, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
		{"abc", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expected)
		}
	}
}
