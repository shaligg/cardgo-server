package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordLevelClearInTx 在结算事务中原子记录一次通关。
//
// MySQL 的唯一键冲突更新会串行化同一玩家同一关卡的并发结算，
// 事务回滚时通关次数也会随奖励一起回滚。
func (r *DBPlayerRepository) RecordLevelClearInTx(ctx context.Context, tx *gorm.DB, uid string, levelID int64) (PlayerLevelProgress, error) {
	if tx == nil {
		return PlayerLevelProgress{}, fmt.Errorf("transaction is nil")
	}
	if uid == "" || levelID <= 0 {
		return PlayerLevelProgress{}, fmt.Errorf("invalid level progress uid=%s level_id=%d", uid, levelID)
	}

	now := time.Now()
	row := model.PlayerLevelProgress{
		UID:            uid,
		LevelID:        levelID,
		ClearCount:     1,
		FirstClearedAt: now,
		LastClearedAt:  now,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}, {Name: "level_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"clear_count":     gorm.Expr("clear_count + 1"),
			"last_cleared_at": now,
		}),
	}).Create(&row).Error; err != nil {
		return PlayerLevelProgress{}, fmt.Errorf("record level clear: %w", err)
	}
	if err := tx.WithContext(ctx).Where("uid = ? AND level_id = ?", uid, levelID).Take(&row).Error; err != nil {
		return PlayerLevelProgress{}, fmt.Errorf("query level progress after clear: %w", err)
	}
	return toDomainPlayerLevelProgress(row), nil
}

func toDomainPlayerLevelProgress(row model.PlayerLevelProgress) PlayerLevelProgress {
	return PlayerLevelProgress{
		UID:            row.UID,
		LevelID:        row.LevelID,
		ClearCount:     row.ClearCount,
		FirstClearedAt: row.FirstClearedAt.Unix(),
		LastClearedAt:  row.LastClearedAt.Unix(),
	}
}
