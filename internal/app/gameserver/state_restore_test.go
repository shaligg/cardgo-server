package gameserver

import (
	"context"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

type restorePlayerRepo struct {
	player repo.Player
}

func (r restorePlayerRepo) GetByUID(context.Context, string) (repo.Player, error) {
	return r.player, nil
}

func (restorePlayerRepo) ChangeGold(context.Context, string, int64, int64, string, string) (repo.Player, error) {
	return repo.Player{}, nil
}

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

func TestRestoreStateLoadsOfficialPlayerDataWhenMemoryMisses(t *testing.T) {
	online := state.NewOnlineState()
	restore := buildRestoreStateCallback(online, restorePlayerRepo{
		player: repo.Player{UID: "u1", Level: 3, Gold: 150},
	})

	data, ok := restore(context.Background(), "u1")
	if !ok || data["level"] != 3 || data["gold"] != int64(150) {
		t.Fatalf("restored data = %+v, ok=%v", data, ok)
	}
	if current, exists := online.Get("u1"); !exists || current.Data["gold"] != int64(150) {
		t.Fatalf("online state = %+v, exists=%v", current, exists)
	}
}
