package state

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type recordingSnapshotStore struct {
	mu        sync.Mutex
	snapshots map[string]Snapshot
}

type recordingOwnerReconciler struct {
	calls int
}

func (r *recordingOwnerReconciler) ReconcileOwners(ctx context.Context) {
	r.calls++
}

func (s *recordingSnapshotStore) SaveSnapshot(ctx context.Context, snapshot Snapshot) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshots == nil {
		s.snapshots = make(map[string]Snapshot)
	}
	s.snapshots[snapshot.UID] = snapshot
	return nil
}

func (s *recordingSnapshotStore) LoadSnapshot(ctx context.Context, uid string) (Snapshot, bool, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.snapshots[uid]
	return snapshot, ok, nil
}

func (s *recordingSnapshotStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.snapshots)
}

func TestFlushWorkerStopPersistsAllQueuedTasks(t *testing.T) {
	queue := NewMemoryFlushQueue(10)
	online := NewOnlineState()
	store := &recordingSnapshotStore{}
	for i := 0; i < 5; i++ {
		uid := fmt.Sprintf("u%d", i)
		online.Set(PlayerState{UID: uid, Version: 1, Data: map[string]interface{}{"gold": i}})
		if err := queue.Enqueue(context.Background(), FlushTask{UID: uid}); err != nil {
			t.Fatalf("enqueue %s: %v", uid, err)
		}
	}

	worker := NewFlushWorker(queue, online, store, FlushWorkerOptions{
		BatchSize: 2,
		Interval:  time.Hour,
		MaxRetry:  1,
	})
	worker.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if queue.Len() != 0 {
		t.Fatalf("queue length = %d, want 0", queue.Len())
	}
	if store.count() != 5 {
		t.Fatalf("saved snapshots = %d, want 5", store.count())
	}
}

func TestFlushWorkerProcessCleansExpiredOnlineState(t *testing.T) {
	online := NewOnlineState()
	online.Set(PlayerState{UID: "u1", Version: 1, Data: map[string]interface{}{"gold": 10}})
	online.MarkOffline("u1", -time.Second)
	worker := NewFlushWorker(nil, online, nil, FlushWorkerOptions{CleanupInterval: time.Minute})

	worker.process(context.Background())

	if online.Len() != 0 {
		t.Fatalf("state count = %d, want 0", online.Len())
	}
}

func TestFlushWorkerReusesTickerForOwnerReconciliation(t *testing.T) {
	reconciler := &recordingOwnerReconciler{}
	worker := NewFlushWorker(nil, nil, nil, FlushWorkerOptions{
		OwnerCheckInterval: time.Minute,
		OwnerReconciler:    reconciler,
	})

	worker.process(context.Background())
	worker.process(context.Background())

	if reconciler.calls != 1 {
		t.Fatalf("reconcile calls = %d, want 1", reconciler.calls)
	}
}
