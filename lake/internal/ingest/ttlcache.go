package ingest

import (
	"sync"
	"time"
)

// TTLCache is a map with per-entry expiration. Entries are lazily evicted
// during Set/Has calls and periodically via a background reaper.
type TTLCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
	maxSize int
	stop    chan struct{}
}

// NewTTLCache creates a cache where entries expire after ttl. If the cache
// exceeds maxSize, the oldest entries are evicted on the next Set call.
// A background goroutine reaps expired entries every ttl/2.
func NewTTLCache(ttl time.Duration, maxSize int) *TTLCache {
	c := &TTLCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
		maxSize: maxSize,
		stop:    make(chan struct{}),
	}
	go c.reaper()
	return c
}

// Set adds or refreshes a key.
func (c *TTLCache) Set(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = time.Now()
	if len(c.entries) > c.maxSize {
		c.evictOldestLocked(len(c.entries) - c.maxSize)
	}
}

// Has returns true if the key exists and hasn't expired.
func (c *TTLCache) Has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.entries[key]
	if !ok {
		return false
	}
	if time.Since(t) > c.ttl {
		delete(c.entries, key)
		return false
	}
	return true
}

// Len returns the number of entries (including possibly expired ones).
func (c *TTLCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Stop terminates the background reaper.
func (c *TTLCache) Stop() {
	close(c.stop)
}

func (c *TTLCache) reaper() {
	interval := c.ttl / 2
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stop:
			return
		}
	}
}

func (c *TTLCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, t := range c.entries {
		if now.Sub(t) > c.ttl {
			delete(c.entries, k)
		}
	}
}

func (c *TTLCache) evictOldestLocked(n int) {
	type entry struct {
		key string
		t   time.Time
	}
	// Find the n oldest entries
	for i := 0; i < n; i++ {
		var oldest entry
		first := true
		for k, t := range c.entries {
			if first || t.Before(oldest.t) {
				oldest = entry{k, t}
				first = false
			}
		}
		if !first {
			delete(c.entries, oldest.key)
		}
	}
}
