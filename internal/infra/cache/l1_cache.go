package cache

import (
	"sync"
	"time"
)

type item struct {
	value interface{}
	exp   time.Time
}

// L1Cache 是进程内短 TTL 读缓存。
type L1Cache struct {
	mu    sync.RWMutex
	items map[string]item
}

// NewL1Cache 创建空的进程内缓存。
func NewL1Cache() *L1Cache {
	return &L1Cache{items: make(map[string]item)}
}

// Set 写入缓存值并设置过期时间。
func (c *L1Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = item{value: value, exp: time.Now().Add(ttl)}
}

// Get 返回未过期的缓存值，并在命中过期项时安全删除旧记录。
func (c *L1Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(it.exp) {
		c.mu.Lock()
		// 加写锁后重新确认，避免删除并发 Set 写入的新值。
		if current, exists := c.items[key]; exists && current.exp.Equal(it.exp) && time.Now().After(current.exp) {
			delete(c.items, key)
		}
		c.mu.Unlock()
		return nil, false
	}
	return it.value, true
}

// Delete 主动删除指定缓存项。
func (c *L1Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
