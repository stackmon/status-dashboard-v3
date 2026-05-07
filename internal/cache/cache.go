package cache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu       sync.RWMutex
	items    map[string]entry[V]
	ttl      time.Duration
	maxItems int
	done     chan struct{}
}

func New[V any](ttl time.Duration, maxItems int) *Cache[V] {
	c := &Cache[V]{
		items:    make(map[string]entry[V]),
		ttl:      ttl,
		maxItems: maxItems,
		done:     make(chan struct{}),
	}
	go c.janitor()
	return c
}

func (c *Cache[V]) janitor() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, e := range c.items {
				if now.After(e.expiresAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.done:
			return
		}
	}
}

func (c *Cache[V]) Get(key string) (V, bool) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	if time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	if c.maxItems > 0 && len(c.items) >= c.maxItems {
		// Evict the entry closest to expiration.
		var oldestKey string
		var oldestTime time.Time
		for k, e := range c.items {
			if oldestTime.IsZero() || e.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = e.expiresAt
			}
		}
		delete(c.items, oldestKey)
	}
	c.items[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *Cache[V]) Invalidate(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *Cache[V]) InvalidateAll() {
	c.mu.Lock()
	c.items = make(map[string]entry[V])
	c.mu.Unlock()
}

func (c *Cache[V]) Close() {
	close(c.done)
}
