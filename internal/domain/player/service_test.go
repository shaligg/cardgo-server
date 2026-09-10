package player

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo"
)

type fakePlayerRepository struct {
	player      repo.Player
	getErr      error
	createCalls int
}

func (r *fakePlayerRepository) GetByUID(context.Context, string) (repo.Player, error) {
	return r.player, r.getErr
}

func (r *fakePlayerRepository) CreateIfAbsent(_ context.Context, player repo.Player) (repo.Player, error) {
	r.createCalls++
	r.player = player
	r.getErr = nil
	return player, nil
}

func TestEnsureCreatedInitializesMissingPlayer(t *testing.T) {
	repository := &fakePlayerRepository{getErr: repo.ErrPlayerNotFound}
	service := Service{Repo: repository}

	player, err := service.EnsureCreated(context.Background(), "u1")
	if err != nil {
		t.Fatalf("EnsureCreated: %v", err)
	}
	if player.UID != "u1" || player.Nickname != defaultNickname("u1") || player.AvatarID != defaultPlayerAvatarID || player.Level != defaultPlayerLevel || player.Gold != 0 {
		t.Fatalf("player = %+v", player)
	}
	if repository.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", repository.createCalls)
	}
}

func TestEnsureCreatedKeepsExistingPlayer(t *testing.T) {
	want := repo.Player{UID: "u1", Nickname: "Existing", AvatarID: 9, Level: 4, Gold: 80}
	repository := &fakePlayerRepository{player: want}
	service := Service{Repo: repository}

	got, err := service.EnsureCreated(context.Background(), "u1")
	if err != nil {
		t.Fatalf("EnsureCreated: %v", err)
	}
	if got != want || repository.createCalls != 0 {
		t.Fatalf("player = %+v, create calls = %d", got, repository.createCalls)
	}
}

func TestEnsureCreatedDoesNotHideQueryFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repository := &fakePlayerRepository{getErr: wantErr}
	service := Service{Repo: repository}

	if _, err := service.EnsureCreated(context.Background(), "u1"); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if repository.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", repository.createCalls)
	}
}
