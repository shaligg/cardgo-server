package ws

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	lastHit map[string]time.Time
	minGap  time.Duration
}

func NewRateLimiter(minGap time.Duration) *RateLimiter {
	return &RateLimiter{lastHit: make(map[string]time.Time), minGap: minGap}
}

func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if t, ok := l.lastHit[key]; ok && now.Sub(t) < l.minGap {
		return false
	}
	l.lastHit[key] = now
	return true
}

// Delete 删除连接关闭后不再使用的限流记录。
func (l *RateLimiter) Delete(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.lastHit, key)
}
