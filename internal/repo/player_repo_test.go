package repo

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestPlayerRepo(t *testing.T) (*DBPlayerRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repo := NewDBPlayerRepository(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
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

func TestChangeGoldReturnsFirstResultOnRetry(t *testing.T) {
	repo, _ := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
	if err != nil {
		t.Fatalf("first ChangeGold returned error: %v", err)
	}
	if _, err := repo.ChangeGold(ctx, "u1", -30, 1, "test.consume", "r2"); err != nil {
		t.Fatalf("consume ChangeGold returned error: %v", err)
	}

	retry, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
	if err != nil {
		t.Fatalf("retry ChangeGold returned error: %v", err)
	}
	if retry.Gold != first.Gold {
		t.Fatalf("retry gold = %d, want first result %d", retry.Gold, first.Gold)
	}

	profile, err := repo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID returned error: %v", err)
	}
	if profile.Gold != 70 {
		t.Fatalf("profile gold = %d, want actual balance 70", profile.Gold)
	}
}

func TestChangeGoldWritesIdempotencyAndAssetLog(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
	if err != nil {
		t.Fatalf("first ChangeGold returned error: %v", err)
	}
	if first.Gold != 100 {
		t.Fatalf("first gold = %d, want 100", first.Gold)
	}

	second, err := repo.ChangeGold(ctx, "u1", 100, 1, "test.grant", "r1")
	if err != nil {
		t.Fatalf("retry ChangeGold returned error: %v", err)
	}
	if second.Gold != 100 {
		t.Fatalf("retry gold = %d, want unchanged 100", second.Gold)
	}

	var idempotencyCount int64
	if err := db.Model(&model.IdempotencyRecord{}).Count(&idempotencyCount).Error; err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idempotencyCount != 1 {
		t.Fatalf("idempotency records = %d, want 1", idempotencyCount)
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

	var idempotencyCount int64
	if err := db.Model(&model.IdempotencyRecord{}).Count(&idempotencyCount).Error; err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idempotencyCount != 0 {
		t.Fatalf("idempotency records = %d, want 0", idempotencyCount)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("asset logs = %d, want 0", logCount)
	}
}

func TestChangeInventoryItemWritesIdempotencyAndAssetLog(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
	if err != nil {
		t.Fatalf("first ChangeInventoryItem returned error: %v", err)
	}
	if first.Count != 5 {
		t.Fatalf("first count = %d, want 5", first.Count)
	}

	second, err := repo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
	if err != nil {
		t.Fatalf("retry ChangeInventoryItem returned error: %v", err)
	}
	if second.Count != 5 {
		t.Fatalf("retry count = %d, want unchanged 5", second.Count)
	}

	var idempotencyCount int64
	if err := db.Model(&model.IdempotencyRecord{}).Where("uid = ? AND action = ?", "u1", "test.grant_material:10001").Count(&idempotencyCount).Error; err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idempotencyCount != 1 {
		t.Fatalf("idempotency records = %d, want 1", idempotencyCount)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ? AND item_id = ?", "u1", int64(10001)).Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("asset logs = %d, want 1", logCount)
	}
}

func TestChangeInventoryItemReturnsFirstResultOnRetry(t *testing.T) {
	repo, _ := newTestPlayerRepo(t)
	ctx := context.Background()

	first, err := repo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
	if err != nil {
		t.Fatalf("first ChangeInventoryItem returned error: %v", err)
	}
	if _, err := repo.ChangeInventoryItem(ctx, "u1", 10001, -2, "test.consume_material", "ir2"); err != nil {
		t.Fatalf("consume ChangeInventoryItem returned error: %v", err)
	}

	retry, err := repo.ChangeInventoryItem(ctx, "u1", 10001, 5, "test.grant_material", "ir1")
	if err != nil {
		t.Fatalf("retry ChangeInventoryItem returned error: %v", err)
	}
	if retry.Count != first.Count {
		t.Fatalf("retry count = %d, want first result %d", retry.Count, first.Count)
	}

	items, err := repo.GetInventory(ctx, "u1")
	if err != nil {
		t.Fatalf("GetInventory returned error: %v", err)
	}
	if len(items) != 1 || items[0].Count != 3 {
		t.Fatalf("inventory = %+v, want actual count 3", items)
	}
}

func TestChangeInventoryItemInsufficientDoesNotWriteSideEffects(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()

	_, err := repo.ChangeInventoryItem(ctx, "u1", 10001, -1, "test.consume_material", "ir3")
	if err != ErrInsufficientItem {
		t.Fatalf("err = %v, want ErrInsufficientItem", err)
	}

	var idempotencyCount int64
	if err := db.Model(&model.IdempotencyRecord{}).Where("uid = ?", "u1").Count(&idempotencyCount).Error; err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	if idempotencyCount != 0 {
		t.Fatalf("idempotency records = %d, want 0", idempotencyCount)
	}

	var logCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ?", "u1").Count(&logCount).Error; err != nil {
		t.Fatalf("count asset log: %v", err)
	}
	if logCount != 0 {
		t.Fatalf("asset logs = %d, want 0", logCount)
	}
}

func TestUpgradeCardInTxWithGoldCostIsAtomicAndIdempotent(t *testing.T) {
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
		card, err = repo.UpgradeCardInTx(ctx, tx, "u1", 10001, 5, "card-r1")
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

	var retry PlayerCard
	var retryPlayer Player
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		retryPlayer, err = repo.ChangeGoldInTx(ctx, tx, "u1", -50, 1, "card.upgrade", "card-r1")
		if err != nil {
			return err
		}
		retry, err = repo.UpgradeCardInTx(ctx, tx, "u1", 10001, 5, "card-r1")
		return err
	})
	if err != nil {
		t.Fatalf("retry upgrade transaction returned error: %v", err)
	}
	if retry.Level != 2 {
		t.Fatalf("retry card level = %d, want first result level 2", retry.Level)
	}
	if retryPlayer.Gold != 50 {
		t.Fatalf("retry player gold = %d, want 50", retryPlayer.Gold)
	}
	player, err = repo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID after retry: %v", err)
	}
	if player.Gold != 50 {
		t.Fatalf("gold after retry = %d, want unchanged 50", player.Gold)
	}

	var upgradeLogCount int64
	if err := db.Model(&model.AssetLog{}).Where("uid = ? AND reason = ? AND req_id = ?", "u1", "card.upgrade", "card-r1").Count(&upgradeLogCount).Error; err != nil {
		t.Fatalf("count upgrade asset logs: %v", err)
	}
	if upgradeLogCount != 1 {
		t.Fatalf("upgrade asset log count = %d, want 1", upgradeLogCount)
	}
}
