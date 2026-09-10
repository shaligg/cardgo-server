package gameserver

import (
	"context"
	"testing"

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

func TestRestoreStateLoadsOfficialPlayerData(t *testing.T) {
	restore := buildRestoreStateCallback(restorePlayerRepo{
		player: repo.Player{UID: "u1", Level: 3, Gold: 150},
	})

	data, ok := restore(context.Background(), "u1")
	if !ok || data["level"] != 3 || data["gold"] != int64(150) {
		t.Fatalf("restored data = %+v, ok=%v", data, ok)
	}
}
