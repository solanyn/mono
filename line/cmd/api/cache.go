package main

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

type responseCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	maxSize int
	ttl     time.Duration
}

func newResponseCache(maxSize int, ttl time.Duration) *responseCache {
	c := &responseCache{
		entries: make(map[string]cacheEntry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
	go c.evictLoop()
	return c
}

func (c *responseCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (c *responseCache) Set(key string, data []byte) {
	c.mu.Lock()
	if len(c.entries) >= c.maxSize {
		oldest := ""
		oldestTime := time.Now().Add(time.Hour)
		for k, v := range c.entries {
			if v.expiresAt.Before(oldestTime) {
				oldest = k
				oldestTime = v.expiresAt
			}
		}
		if oldest != "" {
			delete(c.entries, oldest)
		}
	}
	c.entries[key] = cacheEntry{data: data, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *responseCache) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, v := range c.entries {
			if now.After(v.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}
