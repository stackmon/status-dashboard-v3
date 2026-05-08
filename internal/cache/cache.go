package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry[V any] struct {
	key       string
	value     V
	expiresAt time.Time
}

type Cache[V any] struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	order    *list.List
	ttl      time.Duration
	maxItems int
	done     chan struct{}
}

func New[V any](ttl time.Duration, maxItems int) *Cache[V] {
	c := &Cache[V]{
		items:    make(map[string]*list.Element),
		order:    list.New(),
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
			for el := c.order.Front(); el != nil; {
				e, _ := el.Value.(*entry[V])
				if now.After(e.expiresAt) {
					next := el.Next()
					c.order.Remove(el)
					delete(c.items, e.key)
					el = next
				} else {
					break
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
	el, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		var zero V
		return zero, false
	}

	e, _ := el.Value.(*entry[V])
	if time.Now().After(e.expiresAt) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *Cache[V]) Set(key string, value V) {
	c.mu.Lock()
	if el, exists := c.items[key]; exists {
		c.order.Remove(el)
		delete(c.items, key)
	} else if c.maxItems > 0 && len(c.items) >= c.maxItems {
		oldest := c.order.Front()
		if oldest != nil {
			e, _ := oldest.Value.(*entry[V])
			c.order.Remove(oldest)
			delete(c.items, e.key)
		}
	}
	e := &entry[V]{key: key, value: value, expiresAt: time.Now().Add(c.ttl)}
	c.items[key] = c.order.PushBack(e)
	c.mu.Unlock()
}

func (c *Cache[V]) Invalidate(key string) {
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.order.Remove(el)
		delete(c.items, key)
	}
	c.mu.Unlock()
}

func (c *Cache[V]) InvalidateAll() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
	c.mu.Unlock()
}

func (c *Cache[V]) Close() {
	close(c.done)
}
