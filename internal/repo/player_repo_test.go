package repo

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func newTestPlayerRepo(t *testing.T) (*DBPlayerRepository, *gorm.DB) {
	t.Helper()
	db := testdb.OpenGame(t)
	repo := NewDBPlayerRepository(db)
	return repo, db
}

func TestMigrateCreatesPlayerWorkshopTable(t *testing.T) {
	_, db := newTestPlayerRepo(t)
	if !db.Migrator().HasTable(&model.PlayerWorkshop{}) {
		t.Fatalf("player_workshops table was not migrated")
	}
}

func TestMigrateCreatesPlayerFacilityTable(t *testing.T) {
	_, db := newTestPlayerRepo(t)
	if !db.Migrator().HasTable(&model.PlayerFacility{}) {
		t.Fatalf("player_facilities table was not migrated")
	}
}

func TestChangeGoldWritesAssetLog(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
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
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	_, err := repo.ChangeGold(ctx, "u1", -1, 1, "test.consume", "r2")
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
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
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
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	_, err := repo.ChangeInventoryItem(ctx, "u1", 10001, -1, "test.consume_material", "ir3")
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

func TestUpgradeCardInTxWithGoldCostIsAtomic(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()
	if err := repo.EnsureDefaultCards(ctx, "u1", []int64{10001}); err != nil {
		t.Fatalf("EnsureDefaultCards: %v", err)
	}
	if _, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "gold-r1"); err != nil {
		t.Fatalf("grant gold: %v", err)
	}

	var card PlayerCard
	var playerAfterUpgrade Player
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		playerAfterUpgrade, err = repo.ChangeGoldInTx(ctx, tx, "u1", -50, 1, "card.upgrade", "card-r1")
		if err != nil {
			return err
		}
		card, err = repo.UpgradeCardInTx(ctx, tx, "u1", 10001, 5)
		return err
	})
	if err != nil {
		t.Fatalf("upgrade transaction returned error: %v", err)
	}
	if card.Level != 2 {
		t.Fatalf("card level = %d, want 2", card.Level)
	}
	if playerAfterUpgrade.Gold != 50 {
		t.Fatalf("player gold in result = %d, want 50", playerAfterUpgrade.Gold)
	}
	player, err := repo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 50 {
		t.Fatalf("gold = %d, want 50", player.Gold)
	}

	var upgradeLogCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ? AND reason = ? AND req_id = ?", "u1", "card.upgrade", "card-r1").Count(&upgradeLogCount).Error; err != nil {
		t.Fatalf("count upgrade asset logs: %v", err)
	}
	if upgradeLogCount != 1 {
		t.Fatalf("upgrade asset log count = %d, want 1", upgradeLogCount)
	}
}
