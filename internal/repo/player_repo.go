package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

// DBPlayerRepository 是基于 GORM 的玩家基础资料仓储。
type DBPlayerRepository struct {
	db *gorm.DB
}

// NewDBPlayerRepository 创建玩家基础资料仓储。
func NewDBPlayerRepository(db *gorm.DB) *DBPlayerRepository {
	return &DBPlayerRepository{db: db}
}

// GetByUID 查询玩家基础数据；玩家不存在时创建默认数据。
func (r *DBPlayerRepository) GetByUID(ctx context.Context, uid string) (Player, error) {
	row, err := getOrCreatePlayer(ctx, r.db.WithContext(ctx), uid)
	if err != nil {
		return Player{}, err
	}
	return toDomainPlayer(row), nil
}

// getOrCreatePlayer 在当前事务中查询或创建玩家默认数据。
//
// 资产仓储也会调用此函数，因为金币实际存放在玩家主表。
func getOrCreatePlayer(ctx context.Context, tx *gorm.DB, uid string) (model.Player, error) {
	var row model.Player
	err := tx.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Player{}, fmt.Errorf("query player: %w", err)
	}

	row = model.Player{UID: uid, Level: 1, Gold: 0}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return model.Player{}, fmt.Errorf("create player: %w", err)
	}
	return row, nil
}

func toDomainPlayer(row model.Player) Player {
	return Player{UID: row.UID, Level: row.Level, Gold: row.Gold}
}

// playerExists 查询社交关系目标玩家是否存在。
func playerExists(ctx context.Context, db *gorm.DB, uid string) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&model.Player{}).Where("uid = ?", uid).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return count > 0, nil
}

// playerLevels 批量读取社交列表展示需要的玩家等级。
func playerLevels(ctx context.Context, db *gorm.DB, uids []string) (map[string]int, error) {
	levels := make(map[string]int, len(uids))
	if len(uids) == 0 {
		return levels, nil
	}
	var players []model.Player
	if err := db.WithContext(ctx).Where("uid IN ?", uids).Find(&players).Error; err != nil {
		return nil, fmt.Errorf("query player profiles: %w", err)
	}
	for _, player := range players {
		levels[player.UID] = player.Level
	}
	return levels, nil
}
