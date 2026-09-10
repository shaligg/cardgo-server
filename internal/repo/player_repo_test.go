package repo

import (
	"context"
	"testing"
	"time"

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

func TestMigrateCreatesPlayerLevelProgressTable(t *testing.T) {
	_, db := newTestPlayerRepo(t)
	if !db.Migrator().HasTable(&model.PlayerLevelProgress{}) {
		t.Fatalf("player_level_progresses table was not migrated")
	}
}

func TestLevelProgressRepositoryCreatesAndUpdatesCalculatedProgress(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()
	now := time.Now().Unix()
	progress := PlayerLevelProgress{
		UID:            "u1",
		LevelID:        1,
		ClearCount:     1,
		FirstClearedAt: now,
		LastClearedAt:  now,
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := repo.GetLevelProgressInTx(ctx, tx, "u1", 1); err != ErrLevelProgressNotFound {
			t.Fatalf("missing progress error = %v, want ErrLevelProgressNotFound", err)
		}
		return repo.CreateLevelProgressInTx(ctx, tx, progress)
	}); err != nil {
		t.Fatalf("create progress: %v", err)
	}

	progress.ClearCount = 2
	progress.LastClearedAt = now + 1
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.UpdateLevelProgressInTx(ctx, tx, progress)
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	var stored model.PlayerLevelProgress
	if err := db.Where("uid = ? AND level_id = ?", "u1", 1).Take(&stored).Error; err != nil {
		t.Fatalf("query stored level progress: %v", err)
	}
	if stored.ClearCount != 2 || stored.FirstClearedAt.Unix() != now || stored.LastClearedAt.Unix() != now+1 {
		t.Fatalf("stored progress = %+v, want calculated values", stored)
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

func TestCardRepositoryReadsAndUpdatesCalculatedCardInTx(t *testing.T) {
	repo, db := newTestPlayerRepo(t)
	ctx := context.Background()
	if err := repo.EnsureDefaultCards(ctx, "u1", []int64{10001}); err != nil {
		t.Fatalf("EnsureDefaultCards: %v", err)
	}

	var card PlayerCard
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		card, err = repo.GetCardInTx(ctx, tx, "u1", 10001)
		if err != nil {
			return err
		}
		card.Level++
		return repo.UpdateCardInTx(ctx, tx, card)
	})
	if err != nil {
		t.Fatalf("update card transaction returned error: %v", err)
	}
	if card.Level != 2 {
		t.Fatalf("card level = %d, want 2", card.Level)
	}
	cards, err := repo.GetCards(ctx, "u1")
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if len(cards) != 1 || cards[0].Level != 2 {
		t.Fatalf("stored cards = %+v, want level 2", cards)
	}
}
