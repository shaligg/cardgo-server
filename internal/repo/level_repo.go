package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

// DBLevelProgressRepository 是基于 GORM 的关卡进度仓储。
type DBLevelProgressRepository struct{}

// GetLevelProgressInTx 在结算事务中读取玩家的关卡进度。
func (r *DBLevelProgressRepository) GetLevelProgressInTx(ctx context.Context, tx *gorm.DB, uid string, levelID int64) (PlayerLevelProgress, error) {
	if tx == nil {
		return PlayerLevelProgress{}, fmt.Errorf("transaction is nil")
	}
	if uid == "" || levelID <= 0 {
		return PlayerLevelProgress{}, fmt.Errorf("invalid level progress uid=%s level_id=%d", uid, levelID)
	}

	var row model.PlayerLevelProgress
	if err := tx.WithContext(ctx).Where("uid = ? AND level_id = ?", uid, levelID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PlayerLevelProgress{}, ErrLevelProgressNotFound
		}
		return PlayerLevelProgress{}, fmt.Errorf("query level progress: %w", err)
	}
	return toDomainPlayerLevelProgress(row), nil
}

// CreateLevelProgressInTx 创建业务层已经计算完成的关卡进度。
func (r *DBLevelProgressRepository) CreateLevelProgressInTx(ctx context.Context, tx *gorm.DB, progress PlayerLevelProgress) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	row, err := levelProgressModel(progress)
	if err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create level progress: %w", err)
	}
	return nil
}

// UpdateLevelProgressInTx 更新业务层已经计算完成的关卡进度。
func (r *DBLevelProgressRepository) UpdateLevelProgressInTx(ctx context.Context, tx *gorm.DB, progress PlayerLevelProgress) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	row, err := levelProgressModel(progress)
	if err != nil {
		return err
	}
	result := tx.WithContext(ctx).
		Model(&model.PlayerLevelProgress{}).
		Where("uid = ? AND level_id = ?", progress.UID, progress.LevelID).
		Updates(map[string]interface{}{
			"clear_count":      row.ClearCount,
			"first_cleared_at": row.FirstClearedAt,
			"last_cleared_at":  row.LastClearedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update level progress: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrLevelProgressNotFound
	}
	return nil
}

func levelProgressModel(progress PlayerLevelProgress) (model.PlayerLevelProgress, error) {
	if progress.UID == "" || progress.LevelID <= 0 || progress.ClearCount <= 0 {
		return model.PlayerLevelProgress{}, fmt.Errorf("invalid level progress uid=%s level_id=%d clear_count=%d", progress.UID, progress.LevelID, progress.ClearCount)
	}
	return model.PlayerLevelProgress{
		UID:            progress.UID,
		LevelID:        progress.LevelID,
		ClearCount:     progress.ClearCount,
		FirstClearedAt: time.Unix(progress.FirstClearedAt, 0),
		LastClearedAt:  time.Unix(progress.LastClearedAt, 0),
	}, nil
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
