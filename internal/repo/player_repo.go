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

// WithTx 返回绑定到指定事务的玩家资料仓储。
func (r *DBPlayerRepository) WithTx(tx *gorm.DB) *DBPlayerRepository {
	return &DBPlayerRepository{db: tx}
}

// GetByUID 只查询玩家基础数据，不在读路径隐式建号。
func (r *DBPlayerRepository) GetByUID(ctx context.Context, uid string) (Player, error) {
	row, err := getPlayer(ctx, r.db, uid)
	if err != nil {
		return Player{}, err
	}
	return toDomainPlayer(row), nil
}

// GetByUIDs 批量查询玩家基础资料，返回值按 UID 建立索引。
func (r *DBPlayerRepository) GetByUIDs(ctx context.Context, uids []string) (map[string]Player, error) {
	playersByUID := make(map[string]Player, len(uids))
	if len(uids) == 0 {
		return playersByUID, nil
	}
	var rows []model.Player
	if err := r.db.WithContext(ctx).Where("uid IN ?", uids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query players: %w", err)
	}
	for _, row := range rows {
		playersByUID[row.UID] = toDomainPlayer(row)
	}
	return playersByUID, nil
}

// LockByUID 锁定玩家基础行，串行化同一玩家跨节点的公共数据变更。
func (r *DBPlayerRepository) LockByUID(ctx context.Context, uid string) error {
	var row model.Player
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("uid").Where("uid = ?", uid).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPlayerNotFound
	}
	if err != nil {
		return fmt.Errorf("lock player: %w", err)
	}
	return nil
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
