package handler

import (
	"context"
	"encoding/json"
	"testing"

	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	"github.com/bigfish/go_orm_1/internal/game/asset"
	playergame "github.com/bigfish/go_orm_1/internal/game/player"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBizDispatcherIgnoresPayloadUID(t *testing.T) {
	router := NewRouter()
	router.Register(9001, func(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
		return targetUID, nil
	})
	dispatcher := NewDispatcher(router, nil, nil)

	resp, err := dispatcher.Handle(context.Background(), "auth_uid", 9001, json.RawMessage(`{"uid":"evil_uid"}`))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if resp != "auth_uid" {
		t.Fatalf("target uid = %v, want auth_uid", resp)
	}
}

func TestPlayerHandlerIgnoresPayloadUIDForWrite(t *testing.T) {
	dbRepo, db := newUIDSecurityRepo(t)
	items, err := gamedata.NewCatalog([]gamedata.ItemConfig{
		{ItemID: gamedata.ItemIDGold, Key: "gold", StorageType: gamedata.StoragePlayerField, StorageKey: "gold", Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	playerService := playergame.Service{
		Repo: dbRepo,
		Assets: asset.Service{
			Items:     items,
			Players:   dbRepo,
			Inventory: dbRepo,
		},
	}
	handler := &BizHandler{PlayerService: playerService}

	_, bizErr := handler.PlayerAddGold(context.Background(), "auth_uid", json.RawMessage(`{"uid":"evil_uid","delta":10,"req_id":"r1"}`))
	if bizErr != nil {
		t.Fatalf("AddGold returned error: %v", bizErr)
	}
	authPlayer, err := dbRepo.GetByUID(context.Background(), "auth_uid")
	if err != nil {
		t.Fatalf("GetByUID auth_uid: %v", err)
	}
	if authPlayer.Gold != 10 {
		t.Fatalf("auth_uid gold = %d, want 10", authPlayer.Gold)
	}

	var evilRows int64
	if err := db.Model(&model.Player{}).Where("uid = ?", "evil_uid").Count(&evilRows).Error; err != nil {
		t.Fatalf("count evil_uid: %v", err)
	}
	if evilRows != 0 {
		t.Fatalf("evil_uid rows = %d, want 0", evilRows)
	}
}

func newUIDSecurityRepo(t *testing.T) (*repo.DBPlayerRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	dbRepo := repo.NewDBPlayerRepository(db)
	if err := dbRepo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return dbRepo, db
}
