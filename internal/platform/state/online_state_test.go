package state

import (
	"testing"
	"time"
)

func TestOnlineStateKeepsOfflineStateUntilExpiry(t *testing.T) {
	online := NewOnlineState()
	online.Set(PlayerState{UID: "u1", Version: 1, Data: map[string]interface{}{"gold": int64(10)}})
	online.MarkOffline("u1", time.Minute)

	st, ok := online.Get("u1")
	if !ok || st.ExpiresAt.IsZero() {
		t.Fatalf("offline state = %+v, ok=%v", st, ok)
	}

	online.MarkOnline("u1")
	st, ok = online.Get("u1")
	if !ok || !st.ExpiresAt.IsZero() {
		t.Fatalf("restored state = %+v, ok=%v", st, ok)
	}
}

func TestOnlineStateDeletesExpiredOfflineState(t *testing.T) {
	online := NewOnlineState()
	online.Set(PlayerState{UID: "u1", Version: 1, Data: map[string]interface{}{"gold": int64(10)}})
	online.MarkOffline("u1", time.Minute)

	if deleted := online.DeleteExpired(time.Now().Add(2 * time.Minute)); deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if online.Len() != 0 {
		t.Fatalf("state count = %d, want 0", online.Len())
	}
}
