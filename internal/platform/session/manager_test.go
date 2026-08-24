package session

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBindWithinLimitIsAtomic(t *testing.T) {
	manager := NewMemoryManager()
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, ok, err := manager.BindWithinLimit(context.Background(), Session{
				UID:    fmt.Sprintf("u%d", index),
				ConnID: fmt.Sprintf("c%d", index),
			}, 8)
			if err != nil {
				t.Errorf("BindWithinLimit returned error: %v", err)
				return
			}
			if ok {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()

	count, err := manager.Count(context.Background())
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if accepted.Load() != 8 || count != 8 {
		t.Fatalf("accepted=%d count=%d, want 8", accepted.Load(), count)
	}
}

func TestBindWithinLimitAllowsReplacingExistingUID(t *testing.T) {
	manager := NewMemoryManager()
	if _, accepted, err := manager.BindWithinLimit(context.Background(), Session{UID: "u1", ConnID: "c1"}, 1); err != nil || !accepted {
		t.Fatalf("first bind accepted=%v err=%v", accepted, err)
	}
	oldConnID, accepted, err := manager.BindWithinLimit(context.Background(), Session{UID: "u1", ConnID: "c2"}, 1)
	if err != nil || !accepted || oldConnID != "c1" {
		t.Fatalf("replacement old=%q accepted=%v err=%v", oldConnID, accepted, err)
	}
}
