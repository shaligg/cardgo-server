package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bigfish/go_orm_1/internal/domain/asset"
	playergame "github.com/bigfish/go_orm_1/internal/domain/player"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
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
		Repo: dbRepo.DBPlayerRepository,
		Assets: asset.Service{
			Items:         items,
			PlayerRepo:    dbRepo.DBAssetRepository,
			InventoryRepo: dbRepo.DBAssetRepository,
			Tx:            idb.NewTxManager(db),
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

type uidSecurityRepository struct {
	*repo.DBPlayerRepository
	*repo.DBAssetRepository
}

func newUIDSecurityRepo(t *testing.T) (*uidSecurityRepository, *gorm.DB) {
	t.Helper()
	db := testdb.OpenGame(t)
	dbRepo := &uidSecurityRepository{
		DBPlayerRepository: repo.NewDBPlayerRepository(db),
		DBAssetRepository:  repo.NewDBAssetRepository(db),
	}
	if _, err := dbRepo.CreateIfAbsent(context.Background(), repo.Player{UID: "auth_uid", Nickname: "auth", AvatarID: 1, Level: 1}); err != nil {
		t.Fatalf("create player: %v", err)
	}
	return dbRepo, db
}
