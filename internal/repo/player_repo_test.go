package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func TestPlayerGetDoesNotCreateMissingPlayer(t *testing.T) {
	db := testdb.Open(t, &model.Player{})
	repository := NewDBPlayerRepository(db)

	_, err := repository.GetByUID(context.Background(), "missing")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("err = %v, want ErrPlayerNotFound", err)
	}
	var count int64
	if err := db.Model(&model.Player{}).Count(&count).Error; err != nil {
		t.Fatalf("count players: %v", err)
	}
	if count != 0 {
		t.Fatalf("player count = %d, want 0", count)
	}
}

func TestPlayerCreateIfAbsentPreservesExistingProfile(t *testing.T) {
	db := testdb.Open(t, &model.Player{})
	repository := NewDBPlayerRepository(db)
	ctx := context.Background()

	first, err := repository.CreateIfAbsent(ctx, Player{UID: "u1", Nickname: "First", AvatarID: 3, Level: 2, Gold: 9})
	if err != nil {
		t.Fatalf("first CreateIfAbsent: %v", err)
	}
	second, err := repository.CreateIfAbsent(ctx, Player{UID: "u1", Nickname: "Second", AvatarID: 8, Level: 7, Gold: 99})
	if err != nil {
		t.Fatalf("second CreateIfAbsent: %v", err)
	}
	if second != first {
		t.Fatalf("second player = %+v, want existing %+v", second, first)
	}
}
