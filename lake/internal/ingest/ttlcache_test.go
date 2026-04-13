package ingest

import (
	"testing"
	"time"
)

func TestTTLCache_SetAndHas(t *testing.T) {
	c := NewTTLCache(time.Hour, 1000)
	defer c.Stop()

	c.Set("a")
	c.Set("b")

	if !c.Has("a") {
		t.Error("expected 'a' to be present")
	}
	if !c.Has("b") {
		t.Error("expected 'b' to be present")
	}
	if c.Has("c") {
		t.Error("expected 'c' to be absent")
	}
}

func TestTTLCache_Expiry(t *testing.T) {
	c := NewTTLCache(50*time.Millisecond, 1000)
	defer c.Stop()

	c.Set("ephemeral")
	if !c.Has("ephemeral") {
		t.Fatal("expected key to exist immediately after set")
	}

	time.Sleep(80 * time.Millisecond)

	if c.Has("ephemeral") {
		t.Error("expected key to have expired")
	}
}

func TestTTLCache_MaxSize_EvictsOldest(t *testing.T) {
	c := NewTTLCache(time.Hour, 3)
	defer c.Stop()

	c.Set("first")
	time.Sleep(time.Millisecond)
	c.Set("second")
	time.Sleep(time.Millisecond)
	c.Set("third")

	if c.Len() != 3 {
		t.Fatalf("expected len 3, got %d", c.Len())
	}

	// Adding a 4th should evict "first" (oldest)
	c.Set("fourth")

	if c.Len() != 3 {
		t.Fatalf("expected len 3 after eviction, got %d", c.Len())
	}
	if c.Has("first") {
		t.Error("expected 'first' to be evicted")
	}
	if !c.Has("second") {
		t.Error("expected 'second' to survive")
	}
	if !c.Has("fourth") {
		t.Error("expected 'fourth' to be present")
	}
}

func TestTTLCache_ReaperCleansExpired(t *testing.T) {
	// TTL=50ms, reaper runs every 25ms
	c := NewTTLCache(50*time.Millisecond, 1000)
	defer c.Stop()

	for i := 0; i < 100; i++ {
		c.Set(string(rune('a' + i)))
	}
	if c.Len() != 100 {
		t.Fatalf("expected 100 entries, got %d", c.Len())
	}

	// Wait for expiry + multiple reaper cycles
	time.Sleep(250 * time.Millisecond)

	if c.Len() != 0 {
		t.Errorf("expected 0 entries after reaper, got %d", c.Len())
	}
}

func TestTTLCache_RefreshKeepsAlive(t *testing.T) {
	c := NewTTLCache(80*time.Millisecond, 1000)
	defer c.Stop()

	c.Set("keep")
	time.Sleep(50 * time.Millisecond)
	// Refresh before expiry
	c.Set("keep")
	time.Sleep(50 * time.Millisecond)

	if !c.Has("keep") {
		t.Error("expected refreshed key to still be alive")
	}
}

func TestTTLCache_ConcurrentAccess(t *testing.T) {
	c := NewTTLCache(time.Hour, 100000)
	defer c.Stop()

	done := make(chan struct{})
	// Hammer from multiple goroutines
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 1000; i++ {
				key := string(rune(id*1000 + i))
				c.Set(key)
				c.Has(key)
			}
			done <- struct{}{}
		}(g)
	}
	for g := 0; g < 10; g++ {
		<-done
	}
	// No race detector panic = pass
}
