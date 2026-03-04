package handler

import (
	"sync"
	"time"
)

type cacheEntry struct {
	expiresAt time.Time
	value     interface{}
}

type localTTLCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]cacheEntry
}

func newLocalTTLCache(ttl time.Duration) *localTTLCache {
	return &localTTLCache{
		ttl:  ttl,
		data: make(map[string]cacheEntry),
	}
}

func (c *localTTLCache) get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

func (c *localTTLCache) set(key string, value interface{}) {
	c.mu.Lock()
	c.data[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

func (c *localTTLCache) del(key string) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}
