package cache

import (
	"sync"
	"time"
)

// entry holds a cached count and its expiry time.
type entry struct {
	count  int64
	expiry time.Time
}

// VelocityCache is a thread-safe in-memory cache for velocity counts.
type VelocityCache struct {
	mu      sync.RWMutex
	items   map[string]entry
	ttl     time.Duration
	ticker  *time.Ticker
	stopCh  chan struct{}
}

// NewVelocityCache creates a cache with the given TTL and eviction tick interval.
func NewVelocityCache(ttl, eviction time.Duration) *VelocityCache {
	c := &VelocityCache{
		items:  make(map[string]entry),
		ttl:    ttl,
		ticker: time.NewTicker(eviction),
		stopCh: make(chan struct{}),
	}
	go c.evictLoop()
	return c
}

// Get retrieves a count by key.
func (c *VelocityCache) Get(key string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[key]
	if !ok || time.Now().After(e.expiry) {
		return 0, false
	}
	return e.count, true
}

// Set stores a count for a key.
func (c *VelocityCache) Set(key string, count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = entry{count: count, expiry: time.Now().Add(c.ttl)}
}

// evictLoop periodically removes expired entries.
func (c *VelocityCache) evictLoop() {
	for {
		select {
		case <-c.ticker.C:
			c.evict()
		case <-c.stopCh:
			c.ticker.Stop()
			return
		}
	}
}

func (c *VelocityCache) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.items {
		if now.After(v.expiry) {
			delete(c.items, k)
		}
	}
}

// Stop halts the background eviction goroutine.
func (c *VelocityCache) Stop() {
	close(c.stopCh)
}
