package state

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type SnapshotStore interface {
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
	LoadSnapshot(ctx context.Context, uid string) (Snapshot, bool, error)
}

type FlushObserver interface {
	OnQueueLen(length int)
	OnProcessed()
	OnSaved()
	OnFailed()
	OnRetried()
	OnDropped()
}

type FlushWorkerOptions struct {
	BatchSize int
	Interval  time.Duration
	MaxRetry  int
	Observer  FlushObserver
}

type FlushWorker struct {
	queue     FlushQueue
	online    *OnlineState
	store     SnapshotStore
	batchSize int
	interval  time.Duration
	maxRetry  int
	observer  FlushObserver
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewFlushWorker(queue FlushQueue, online *OnlineState, store SnapshotStore, opts FlushWorkerOptions) *FlushWorker {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 128
	}
	if opts.Interval <= 0 {
		opts.Interval = 200 * time.Millisecond
	}
	if opts.MaxRetry < 0 {
		opts.MaxRetry = 0
	}
	return &FlushWorker{
		queue:     queue,
		online:    online,
		store:     store,
		batchSize: opts.BatchSize,
		interval:  opts.Interval,
		maxRetry:  opts.MaxRetry,
		observer:  opts.Observer,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (w *FlushWorker) Start() {
	go func() {
		defer close(w.doneCh)
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				w.process(context.Background())
				return
			case <-ticker.C:
				w.process(context.Background())
			}
		}
	}()
}

func (w *FlushWorker) Stop(ctx context.Context) error {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})

	select {
	case <-w.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *FlushWorker) process(ctx context.Context) {
	if w.queue == nil || w.online == nil || w.store == nil {
		return
	}
	tasks, err := w.queue.DequeueBatch(ctx, w.batchSize)
	if err != nil || len(tasks) == 0 {
		return
	}
	w.observeQueueLen()

	seen := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.UID == "" {
			continue
		}
		if _, ok := seen[task.UID]; ok {
			continue
		}
		seen[task.UID] = struct{}{}

		w.observeProcessed()

		st, ok := w.online.Get(task.UID)
		if !ok {
			continue
		}
		payload, err := json.Marshal(st.Data)
		if err != nil {
			w.observeFailed()
			w.retryOrDrop(ctx, task)
			continue
		}

		if err := w.store.SaveSnapshot(ctx, Snapshot{
			UID:     st.UID,
			Version: st.Version,
			Payload: payload,
		}); err != nil {
			w.observeFailed()
			w.retryOrDrop(ctx, task)
			continue
		}
		w.observeSaved()
	}
}

func (w *FlushWorker) retryOrDrop(ctx context.Context, task FlushTask) {
	if task.Retry < w.maxRetry {
		task.Retry++
		_ = w.queue.Enqueue(ctx, task)
		w.observeRetried()
		w.observeQueueLen()
		return
	}
	w.observeDropped()
}

func (w *FlushWorker) observeQueueLen() {
	if w.observer != nil && w.queue != nil {
		w.observer.OnQueueLen(w.queue.Len())
	}
}

func (w *FlushWorker) observeProcessed() {
	if w.observer != nil {
		w.observer.OnProcessed()
	}
}

func (w *FlushWorker) observeSaved() {
	if w.observer != nil {
		w.observer.OnSaved()
	}
}

func (w *FlushWorker) observeFailed() {
	if w.observer != nil {
		w.observer.OnFailed()
	}
}

func (w *FlushWorker) observeRetried() {
	if w.observer != nil {
		w.observer.OnRetried()
	}
}

func (w *FlushWorker) observeDropped() {
	if w.observer != nil {
		w.observer.OnDropped()
	}
}
