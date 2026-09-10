package repo

import (
	"context"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func TestLevelProgressRepositoryCreatesAndUpdatesCalculatedProgress(t *testing.T) {
	db := testdb.OpenGame(t)
	progressRepo := &DBLevelProgressRepository{}
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
		if _, err := progressRepo.GetLevelProgressInTx(ctx, tx, "u1", 1); err != ErrLevelProgressNotFound {
			t.Fatalf("missing progress error = %v, want ErrLevelProgressNotFound", err)
		}
		return progressRepo.CreateLevelProgressInTx(ctx, tx, progress)
	}); err != nil {
		t.Fatalf("create progress: %v", err)
	}

	progress.ClearCount = 2
	progress.LastClearedAt = now + 1
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return progressRepo.UpdateLevelProgressInTx(ctx, tx, progress)
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
