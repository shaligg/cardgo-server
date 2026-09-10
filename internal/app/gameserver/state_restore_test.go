package gameserver

import (
	"context"
	"errors"
	"testing"

	playergame "github.com/bigfish/go_orm_1/internal/domain/player"
	"github.com/bigfish/go_orm_1/internal/repo"
)

type restorePlayerRepo struct {
	player    repo.Player
	getErr    error
	createErr error
}

func (r *restorePlayerRepo) GetByUID(context.Context, string) (repo.Player, error) {
	return r.player, r.getErr
}

func (r *restorePlayerRepo) CreateIfAbsent(_ context.Context, player repo.Player) (repo.Player, error) {
	if r.createErr != nil {
		return repo.Player{}, r.createErr
	}
	r.player = player
	r.getErr = nil
	return player, nil
}

func TestPreparePlayerLoadsOfficialPlayerData(t *testing.T) {
	repository := &restorePlayerRepo{player: repo.Player{
		UID: "u1", Nickname: "Captain", AvatarID: 7, Level: 3, Gold: 150,
	}}
	prepare := buildPreparePlayerCallback(playergame.Service{Repo: repository})

	data, err := prepare(context.Background(), "u1")
	if err != nil {
		t.Fatalf("prepare player: %v", err)
	}
	if data["nickname"] != "Captain" || data["avatar_id"] != int64(7) || data["level"] != 3 || data["gold"] != int64(150) {
		t.Fatalf("prepared data = %+v", data)
	}
}

func TestPreparePlayerReturnsInitializationError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := &restorePlayerRepo{getErr: wantErr}
	prepare := buildPreparePlayerCallback(playergame.Service{Repo: repository})

	if _, err := prepare(context.Background(), "u1"); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
