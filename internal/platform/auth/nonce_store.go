package auth

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrReplay = errors.New("nonce replay")

type NonceStore interface {
	ConsumeOnce(ctx context.Context, nonce string, ttl time.Duration) error
}

type MemoryNonceStore struct {
	mu    sync.Mutex
	items map[string]time.Time
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{items: make(map[string]time.Time)}
}

func (s *MemoryNonceStore) ConsumeOnce(ctx context.Context, nonce string, ttl time.Duration) error {
	_ = ctx
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for k, exp := range s.items {
		if exp.Before(now) {
			delete(s.items, k)
		}
	}

	if _, ok := s.items[nonce]; ok {
		return ErrReplay
	}
	s.items[nonce] = now.Add(ttl)
	return nil
}
