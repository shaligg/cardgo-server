package state

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrFlushQueueFull = errors.New("flush queue full")

type FlushTask struct {
	UID       string
	EnqueueAt time.Time
	Retry     int
}

type FlushQueue interface {
	Enqueue(ctx context.Context, task FlushTask) error
	DequeueBatch(ctx context.Context, max int) ([]FlushTask, error)
	Drain(ctx context.Context) error
	Len() int
}

type MemoryFlushQueue struct {
	mu       sync.Mutex
	items    []FlushTask
	capacity int
}

func NewMemoryFlushQueue(capacity int) *MemoryFlushQueue {
	if capacity <= 0 {
		capacity = 10000
	}
	return &MemoryFlushQueue{
		items:    make([]FlushTask, 0, capacity),
		capacity: capacity,
	}
}

func (q *MemoryFlushQueue) Enqueue(ctx context.Context, task FlushTask) error {
	_ = ctx
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= q.capacity {
		return ErrFlushQueueFull
	}
	if task.EnqueueAt.IsZero() {
		task.EnqueueAt = time.Now()
	}
	q.items = append(q.items, task)
	return nil
}

func (q *MemoryFlushQueue) Drain(ctx context.Context) error {
	_ = ctx
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = q.items[:0]
	return nil
}

func (q *MemoryFlushQueue) DequeueBatch(ctx context.Context, max int) ([]FlushTask, error) {
	_ = ctx
	if max <= 0 {
		max = 1
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil, nil
	}

	n := max
	if n > len(q.items) {
		n = len(q.items)
	}
	out := make([]FlushTask, n)
	copy(out, q.items[:n])
	q.items = q.items[n:]
	return out, nil
}

func (q *MemoryFlushQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
