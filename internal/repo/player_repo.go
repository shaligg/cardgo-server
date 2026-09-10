package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBPlayerRepository 是基于 GORM 的玩家基础资料仓储。
type DBPlayerRepository struct {
	db *gorm.DB
}

// NewDBPlayerRepository 创建玩家基础资料仓储。
func NewDBPlayerRepository(db *gorm.DB) *DBPlayerRepository {
	return &DBPlayerRepository{db: db}
}

// GetByUID 只查询玩家基础数据，不在读路径隐式建号。
func (r *DBPlayerRepository) GetByUID(ctx context.Context, uid string) (Player, error) {
	row, err := getPlayer(ctx, r.db, uid)
	if err != nil {
		return Player{}, err
	}
	return toDomainPlayer(row), nil
}

// CreateIfAbsent 幂等创建玩家，并返回数据库中的最终资料。
//
// 默认值由玩家领域传入，Repository 只负责持久化和并发建号去重。
func (r *DBPlayerRepository) CreateIfAbsent(ctx context.Context, player Player) (Player, error) {
	row := model.Player{
		UID:      player.UID,
		Nickname: player.Nickname,
		AvatarID: player.AvatarID,
		Level:    player.Level,
		Gold:     player.Gold,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return Player{}, fmt.Errorf("create player: %w", result.Error)
	}
	return r.GetByUID(ctx, player.UID)
}

// getPlayer 在指定数据库会话中读取玩家，供玩家与资产仓储复用。
func getPlayer(ctx context.Context, tx *gorm.DB, uid string) (model.Player, error) {
	var row model.Player
	err := tx.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
	if err == nil {
		return row, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Player{}, ErrPlayerNotFound
	}
	return model.Player{}, fmt.Errorf("query player: %w", err)
}

func toDomainPlayer(row model.Player) Player {
	return Player{
		UID:      row.UID,
		Nickname: row.Nickname,
		AvatarID: row.AvatarID,
		Level:    row.Level,
		Gold:     row.Gold,
	}
}

// playerExists 查询社交关系目标玩家是否存在。
func playerExists(ctx context.Context, db *gorm.DB, uid string) (bool, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&model.Player{}).Where("uid = ?", uid).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return count > 0, nil
}

// playerProfiles 批量读取社交列表展示需要的玩家基础资料。
func playerProfiles(ctx context.Context, db *gorm.DB, uids []string) (map[string]Player, error) {
	profiles := make(map[string]Player, len(uids))
	if len(uids) == 0 {
		return profiles, nil
	}
	var players []model.Player
	if err := db.WithContext(ctx).Where("uid IN ?", uids).Find(&players).Error; err != nil {
		return nil, fmt.Errorf("query player profiles: %w", err)
	}
	for _, player := range players {
		profiles[player.UID] = toDomainPlayer(player)
	}
	return profiles, nil
}
