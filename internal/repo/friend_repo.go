package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBFriendRepository 是基于 GORM 的好友关系仓储。
//
// 本类型只负责好友关系的查询和持久化，申请方向与状态流转由 globalcore 处理。
type DBFriendRepository struct {
	db *gorm.DB
}

// NewDBFriendRepository 创建好友关系仓储。
func NewDBFriendRepository(db *gorm.DB) *DBFriendRepository {
	return &DBFriendRepository{db: db}
}

// WithTx 返回绑定到指定事务的好友仓储。
func (r *DBFriendRepository) WithTx(tx *gorm.DB) *DBFriendRepository {
	return &DBFriendRepository{db: tx}
}

// CreateFriendRelationData 尝试写入好友关系，并返回本次是否成功创建。
func (r *DBFriendRepository) CreateFriendRelationData(ctx context.Context, uid string, targetUID string, requesterUID string, status string, reqID string) (bool, error) {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	row := model.FriendRelation{
		UIDLow:       uidLow,
		UIDHigh:      uidHigh,
		RequesterUID: requesterUID,
		Status:       status,
		ReqID:        reqID,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return false, fmt.Errorf("create friend relation: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// FindFriendRelation 查询两个玩家之间的关系。
func (r *DBFriendRepository) FindFriendRelation(ctx context.Context, uid string, targetUID string) (FriendRelationRecord, bool, error) {
	return r.findFriendRelation(ctx, uid, targetUID, false)
}

// FindFriendRelationForUpdate 锁定并查询两个玩家之间的关系。
func (r *DBFriendRepository) FindFriendRelationForUpdate(ctx context.Context, uid string, targetUID string) (FriendRelationRecord, bool, error) {
	return r.findFriendRelation(ctx, uid, targetUID, true)
}

// UpdateFriendRelationData 更新好友关系状态和请求 ID。
func (r *DBFriendRepository) UpdateFriendRelationData(ctx context.Context, uid string, targetUID string, status string, reqID string) error {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	if err := r.db.WithContext(ctx).Model(&model.FriendRelation{}).
		Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).
		Updates(map[string]interface{}{"status": status, "req_id": reqID}).Error; err != nil {
		return fmt.Errorf("update friend relation: %w", err)
	}
	return nil
}

// DeleteFriendRelationData 删除好友关系，并返回是否存在可删除记录。
func (r *DBFriendRepository) DeleteFriendRelationData(ctx context.Context, uid string, targetUID string) (bool, error) {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	result := r.db.WithContext(ctx).Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).Delete(&model.FriendRelation{})
	if result.Error != nil {
		return false, fmt.Errorf("delete friend relation: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

// ListFriendRelations 按关系 ID 正向分页查询好友和待处理申请。
func (r *DBFriendRepository) ListFriendRelations(ctx context.Context, uid string, afterID uint64, limit int) ([]FriendRecord, uint64, error) {
	var rows []model.FriendRelation
	query := r.db.WithContext(ctx).Where("(uid_low = ? OR uid_high = ?) AND id > ?", uid, uid, afterID)
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list friend relations: %w", err)
	}
	nextCursor := uint64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		nextCursor = rows[len(rows)-1].ID
	}

	result := make([]FriendRecord, 0, len(rows))
	for _, row := range rows {
		otherUID := row.UIDLow
		if otherUID == uid {
			otherUID = row.UIDHigh
		}
		result = append(result, FriendRecord{
			OtherUID:     otherUID,
			RequesterUID: row.RequesterUID,
			Status:       row.Status,
		})
	}
	return result, nextCursor, nil
}

func (r *DBFriendRepository) findFriendRelation(ctx context.Context, uid string, targetUID string, forUpdate bool) (FriendRelationRecord, bool, error) {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	query := r.db.WithContext(ctx)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row model.FriendRelation
	err := query.Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FriendRelationRecord{}, false, nil
	}
	if err != nil {
		return FriendRelationRecord{}, false, fmt.Errorf("query friend relation: %w", err)
	}
	return FriendRelationRecord{RequesterUID: row.RequesterUID, Status: row.Status}, true, nil
}

func orderedUIDPair(uid string, targetUID string) (string, string) {
	if uid < targetUID {
		return uid, targetUID
	}
	return targetUID, uid
}
