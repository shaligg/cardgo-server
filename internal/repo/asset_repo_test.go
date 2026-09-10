package repo

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func TestChangeGoldWritesAssetLog(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	playerRepo := NewDBPlayerRepository(db)
	ctx := context.Background()
	if _, err := playerRepo.CreateIfAbsent(ctx, Player{UID: "u1", Nickname: "u1", AvatarID: 1, Level: 1}); err != nil {
		t.Fatalf("create player: %v", err)
	}

	first, err := assetRepo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
	if err != nil {
		t.Fatalf("first ChangeGold returned error: %v", err)
	}
	if first.Gold != 100 {
		t.Fatalf("first gold = %d, want 100", first.Gold)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("asset logs = %d, want 1", logCount)
	}
}

func TestChangeGoldInsufficientDoesNotWriteSideEffects(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	playerRepo := NewDBPlayerRepository(db)
	ctx := context.Background()
	if _, err := playerRepo.CreateIfAbsent(ctx, Player{UID: "u1", Nickname: "u1", AvatarID: 1, Level: 1}); err != nil {
		t.Fatalf("create player: %v", err)
	}

	_, err := assetRepo.ChangeGold(ctx, "u1", -1, 1, "test.consume", "r2")
	if err != ErrInsufficientGold {
		t.Fatalf("err = %v, want ErrInsufficientGold", err)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("asset logs = %d, want 0", logCount)
	}
}

func TestChangeInventoryItemWritesAssetLog(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	ctx := context.Background()

	first, err := assetRepo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
	if err != nil {
		t.Fatalf("first ChangeInventoryItem returned error: %v", err)
	}
	if first.Count != 5 {
		t.Fatalf("first count = %d, want 5", first.Count)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ? AND item_id = ?", "u1", int64(10001)).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("asset logs = %d, want 1", logCount)
	}
}

func TestChangeInventoryItemInsufficientDoesNotWriteSideEffects(t *testing.T) {
	db := testdb.OpenGame(t)
	assetRepo := NewDBAssetRepository(db)
	ctx := context.Background()

	_, err := assetRepo.ChangeInventoryItem(ctx, "u1", 10001, -1, "test.consume_material", "ir3")
	if err != ErrInsufficientItem {
		t.Fatalf("err = %v, want ErrInsufficientItem", err)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ?", "u1").Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("asset logs = %d, want 0", logCount)
	}
}
