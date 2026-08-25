package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrReplay = errors.New("nonce replay")

const (
	nonceCleanupThreshold = 2000
	nonceCleanupInterval  = 30 * time.Second
)

type NonceStore interface {
	ConsumeOnce(ctx context.Context, nonce string, ttl time.Duration) error
}

type MemoryNonceStore struct {
	mu          sync.Mutex
	items       map[string]time.Time
	nextCleanup time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{items: make(map[string]time.Time)}
}

func (s *MemoryNonceStore) ConsumeOnce(ctx context.Context, nonce string, ttl time.Duration) error {
	_ = ctx
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if expiresAt, ok := s.items[nonce]; ok {
		if expiresAt.After(now) {
			return ErrReplay
		}
		delete(s.items, nonce)
	}
	s.items[nonce] = now.Add(ttl)

	// 数量达到阈值后最多每 30 秒扫描一次，避免每次验票都遍历全部 nonce。
	if len(s.items) >= nonceCleanupThreshold && !now.Before(s.nextCleanup) {
		for key, expiresAt := range s.items {
			if !expiresAt.After(now) {
				delete(s.items, key)
			}
		}
		s.nextCleanup = now.Add(nonceCleanupInterval)
	}
	return nil
}
