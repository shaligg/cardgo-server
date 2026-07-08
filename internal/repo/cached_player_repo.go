package repo

import (
	"context"
	"sync"
	"time"

	"github.com/bigfish/go_orm_1/internal/infra/cache"
)

type CachedPlayerRepository struct {
	inner PlayerRepository
	l1    *cache.L1Cache
	ttl   time.Duration
	mu    sync.Mutex
}

func NewCachedPlayerRepository(inner PlayerRepository, l1 *cache.L1Cache, ttl time.Duration) *CachedPlayerRepository {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &CachedPlayerRepository{
		inner: inner,
		l1:    l1,
		ttl:   ttl,
	}
}

func (r *CachedPlayerRepository) GetByUID(ctx context.Context, uid string) (Player, error) {
	key := playerCacheKey(uid)
	if r.l1 != nil {
		if v, ok := r.l1.Get(key); ok {
			if p, ok2 := v.(Player); ok2 {
				return p, nil
			}
		}
	}

	// MVP 阶段用互斥占位，后续替换为 singleflight + 分段锁。
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.l1 != nil {
		if v, ok := r.l1.Get(key); ok {
			if p, ok2 := v.(Player); ok2 {
				return p, nil
			}
		}
	}

	p, err := r.inner.GetByUID(ctx, uid)
	if err != nil {
		return Player{}, err
	}
	if r.l1 != nil {
		r.l1.Set(key, p, r.ttl)
	}
	return p, nil
}

func (r *CachedPlayerRepository) ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error) {
	p, err := r.inner.ChangeGold(ctx, uid, delta, itemID, reason, reqID)
	if err != nil {
		return Player{}, err
	}
	r.InvalidatePlayer(uid)
	return p, nil
}

// InvalidatePlayer 删除玩家基础数据缓存。
func (r *CachedPlayerRepository) InvalidatePlayer(uid string) {
	if r.l1 != nil {
		r.l1.Delete(playerCacheKey(uid))
	}
}
