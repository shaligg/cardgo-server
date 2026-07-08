package dispatcher

import (
	"context"
	"sync"
)

type Task func(ctx context.Context) error

type ShardExecutor struct {
	shards int
	locks  []sync.Mutex
}

func NewShardExecutor(shards int) *ShardExecutor {
	if shards <= 0 {
		shards = 1
	}
	return &ShardExecutor{
		shards: shards,
		locks:  make([]sync.Mutex, shards),
	}
}

func (e *ShardExecutor) Submit(ctx context.Context, domain Domain, key string, task Task) error {
	idx := RouteShard(domain, key, e.shards)
	e.locks[idx].Lock()
	defer e.locks[idx].Unlock()
	return task(ctx)
}

func (e *ShardExecutor) Shards() int {
	return e.shards
}
