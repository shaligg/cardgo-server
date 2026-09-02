package state

import (
	"context"
	"testing"
	"time"
)

type countingOwnerReconciler struct {
	calls int
}

func (r *countingOwnerReconciler) ReconcileOwners(context.Context) {
	r.calls++
}

func TestMaintainerCleansExpiredStateAndReconcilesOwner(t *testing.T) {
	now := time.Now()
	online := NewOnlineState()
	online.Set(PlayerState{UID: "u1"})
	online.MarkOffline("u1", time.Millisecond)
	reconciler := &countingOwnerReconciler{}
	maintainer := NewMaintainer(online, MaintainerOptions{
		CleanupInterval:    time.Minute,
		OwnerCheckInterval: time.Minute,
		OwnerReconciler:    reconciler,
	})

	maintainer.maintain(context.Background(), now.Add(time.Second))

	if _, ok := online.Get("u1"); ok {
		t.Fatal("expired online state was not deleted")
	}
	if reconciler.calls != 1 {
		t.Fatalf("reconcile calls = %d, want 1", reconciler.calls)
	}
}

func TestMaintainerHonorsIntervals(t *testing.T) {
	reconciler := &countingOwnerReconciler{}
	maintainer := NewMaintainer(nil, MaintainerOptions{
		CleanupInterval:    time.Minute,
		OwnerCheckInterval: time.Minute,
		OwnerReconciler:    reconciler,
	})
	now := time.Now()

	maintainer.maintain(context.Background(), now)
	maintainer.maintain(context.Background(), now.Add(30*time.Second))
	maintainer.maintain(context.Background(), now.Add(time.Minute))

	if reconciler.calls != 2 {
		t.Fatalf("reconcile calls = %d, want 2", reconciler.calls)
	}
}
