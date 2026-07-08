package cache

import (
	"sync"
	"time"
)

type item struct {
	value interface{}
	exp   time.Time
}

type L1Cache struct {
	mu    sync.RWMutex
	items map[string]item
}

func NewL1Cache() *L1Cache {
	return &L1Cache{items: make(map[string]item)}
}

func (c *L1Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = item{value: value, exp: time.Now().Add(ttl)}
}

func (c *L1Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(it.exp) {
		return nil, false
	}
	return it.value, true
}

func (c *L1Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
