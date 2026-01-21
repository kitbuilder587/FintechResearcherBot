package memory

import (
	"context"
	"sync"
	"time"
)

type item struct {
	value     interface{}
	expiresAt time.Time
}

// Cache - простой in-memory кеш с TTL
type Cache struct {
	mu       sync.RWMutex
	items    map[string]item
	stopChan chan struct{}
	stopped  bool
}

func New() *Cache {
	return NewWithContext(context.Background())
}

func NewWithContext(ctx context.Context) *Cache {
	c := &Cache{
		items:    make(map[string]item),
		stopChan: make(chan struct{}),
	}
	go c.cleanup(ctx)
	return c
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	it, ok := c.items[key]
	if !ok || time.Now().After(it.expiresAt) {
		return nil, false
	}
	return it.value, true
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	c.items[key] = item{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *Cache) Stop() {
	c.mu.Lock()
	if !c.stopped {
		c.stopped = true
		close(c.stopChan)
	}
	c.mu.Unlock()
}

// cleanup чистит просроченные записи раз в 5 минут
// XXX: интервал захардкожен, может стоит вынести в конфиг
func (c *Cache) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.removeExpired()
		}
	}
}

func (c *Cache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, it := range c.items {
		if now.After(it.expiresAt) {
			delete(c.items, k)
		}
	}
}
