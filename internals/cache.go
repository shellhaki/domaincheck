package internals

import (
	"sync"
	"time"

	"shellhaki/domaincheck/models"
)

type cacheEntry struct {
	value   models.DomainResult
	expires time.Time
}

type ResultCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	ttl     time.Duration
	max     int
}

func NewResultCache(ttl time.Duration, maxEntries int) *ResultCache {
	return &ResultCache{entries: make(map[string]cacheEntry), ttl: ttl, max: maxEntries}
}

func (c *ResultCache) Get(key string) (models.DomainResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return models.DomainResult{}, false
	}
	if time.Now().After(entry.expires) {
		delete(c.entries, key)
		return models.DomainResult{}, false
	}
	entry.value.Cached = true
	return entry.value, true
}

func (c *ResultCache) Set(key string, value models.DomainResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, entry := range c.entries {
		if now.After(entry.expires) {
			delete(c.entries, k)
		}
	}
	if _, exists := c.entries[key]; !exists && c.max > 0 && len(c.entries) >= c.max {
		var oldestKey string
		var oldest time.Time
		for k, entry := range c.entries {
			if oldest.IsZero() || entry.expires.Before(oldest) {
				oldestKey, oldest = k, entry.expires
			}
		}
		delete(c.entries, oldestKey)
	}
	value.Cached = false
	c.entries[key] = cacheEntry{value: value, expires: now.Add(c.ttl)}
}
