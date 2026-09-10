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

func TestMaintainerHonorsIntervals(t *testing.T) {
	reconciler := &countingOwnerReconciler{}
	maintainer := NewMaintainer(MaintainerOptions{
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
