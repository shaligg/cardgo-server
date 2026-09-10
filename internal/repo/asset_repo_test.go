package repo

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func TestSaveGoldInTxWritesAssetLog(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	playerRepo := NewDBPlayerRepository(db)
	ctx := context.Background()
	if _, err := playerRepo.CreateIfAbsent(ctx, Player{UID: "u1", Nickname: "u1", AvatarID: 1, Level: 1}); err != nil {
		t.Fatalf("create player: %v", err)
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return assetRepo.SaveGoldInTx(ctx, tx, "u1", 100, 1, 100, "test.grant", "r1")
	})
	if err != nil {
		t.Fatalf("SaveGoldInTx returned error: %v", err)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("asset logs = %d, want 1", logCount)
	}
	player, err := playerRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID returned error: %v", err)
	}
	if player.Gold != 100 {
		t.Fatalf("gold = %d, want 100", player.Gold)
	}
}

func TestSaveInventoryItemInTxWritesAssetLog(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	ctx := context.Background()

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item, err := assetRepo.GetOrCreateInventoryItemInTx(ctx, tx, "u1", 10001)
		if err != nil {
			return err
		}
		item.Count = 5
		return assetRepo.SaveInventoryItemInTx(ctx, tx, item, 5, "test.grant_material", "ir1")
	})
	if err != nil {
		t.Fatalf("SaveInventoryItemInTx returned error: %v", err)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ? AND item_id = ?", "u1", int64(10001)).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("asset logs = %d, want 1", logCount)
	}
}
