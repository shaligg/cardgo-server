package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/state"
)

func TestRestoreStateMarksOfflineStateOnline(t *testing.T) {
	online := state.NewOnlineState()
	online.Set(state.PlayerState{UID: "u1", Version: 1, Data: map[string]interface{}{"gold": int64(10)}})
	online.MarkOffline("u1", time.Minute)

	restore := buildRestoreStateCallback(online, nil)
	data, ok := restore(context.Background(), "u1")
	if !ok || data["gold"] != int64(10) {
		t.Fatalf("restored data = %+v, ok=%v", data, ok)
	}
	st, ok := online.Get("u1")
	if !ok || !st.ExpiresAt.IsZero() {
		t.Fatalf("online state = %+v, ok=%v", st, ok)
	}
}
