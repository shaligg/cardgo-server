package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateFriendRequest 创建一条待审批好友关系。
func (r *DBPlayerRepository) CreateFriendRequest(ctx context.Context, uid string, targetUID string, reqID string) error {
	exists, err := r.playerExists(ctx, targetUID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrSocialPlayerNotFound
	}
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	row := model.FriendRelation{
		UIDLow:       uidLow,
		UIDHigh:      uidHigh,
		RequesterUID: uid,
		Status:       FriendStatusPending,
		ReqID:        reqID,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("create friend request: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var existing model.FriendRelation
	if err := r.db.WithContext(ctx).Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).Take(&existing).Error; err != nil {
		return fmt.Errorf("query existing friend relation: %w", err)
	}
	if existing.Status == FriendStatusAccepted {
		return ErrAlreadyFriends
	}
	return ErrFriendRequestExists
}

// ApproveFriendRequest 把目标玩家发起的申请转为好友关系。
func (r *DBPlayerRepository) ApproveFriendRequest(ctx context.Context, uid string, targetUID string, reqID string) error {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.FriendRelation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendRequestNotFound
		}
		if err != nil {
			return fmt.Errorf("query friend request: %w", err)
		}
		if row.Status == FriendStatusAccepted {
			return ErrAlreadyFriends
		}
		if row.RequesterUID != targetUID {
			return ErrFriendRequestNotFound
		}
		return tx.Model(&row).Updates(map[string]interface{}{
			"status": FriendStatusAccepted,
			"req_id": reqID,
		}).Error
	})
}

// DeleteFriendRelation 删除好友关系或尚未处理的申请。
func (r *DBPlayerRepository) DeleteFriendRelation(ctx context.Context, uid string, targetUID string) error {
	uidLow, uidHigh := orderedUIDPair(uid, targetUID)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.FriendRelation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uid_low = ? AND uid_high = ?", uidLow, uidHigh).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFriendRelationNotFound
		}
		if err != nil {
			return fmt.Errorf("query friend relation: %w", err)
		}
		if err := tx.Delete(&row).Error; err != nil {
			return fmt.Errorf("delete friend relation: %w", err)
		}
		return nil
	})
}

// ListFriendRelations 按关系 ID 正向分页查询好友和待处理申请。
func (r *DBPlayerRepository) ListFriendRelations(ctx context.Context, uid string, afterID uint64, limit int) ([]FriendRecord, uint64, error) {
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

	otherUIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.UIDLow == uid {
			otherUIDs = append(otherUIDs, row.UIDHigh)
		} else {
			otherUIDs = append(otherUIDs, row.UIDLow)
		}
	}
	levels, err := r.playerLevels(ctx, otherUIDs)
	if err != nil {
		return nil, 0, err
	}

	result := make([]FriendRecord, 0, len(rows))
	for _, row := range rows {
		otherUID := row.UIDLow
		if otherUID == uid {
			otherUID = row.UIDHigh
		}
		level := levels[otherUID]
		if level == 0 {
			level = 1
		}
		result = append(result, FriendRecord{
			OtherUID:     otherUID,
			RequesterUID: row.RequesterUID,
			Level:        level,
			Nickname:     otherUID,
			Status:       row.Status,
		})
	}
	return result, nextCursor, nil
}

func orderedUIDPair(uid string, targetUID string) (string, string) {
	if uid < targetUID {
		return uid, targetUID
	}
	return targetUID, uid
}

func (r *DBPlayerRepository) playerExists(ctx context.Context, uid string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Player{}).Where("uid = ?", uid).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return count > 0, nil
}

func (r *DBPlayerRepository) playerLevels(ctx context.Context, uids []string) (map[string]int, error) {
	levels := make(map[string]int, len(uids))
	if len(uids) == 0 {
		return levels, nil
	}
	var players []model.Player
	if err := r.db.WithContext(ctx).Where("uid IN ?", uids).Find(&players).Error; err != nil {
		return nil, fmt.Errorf("query player profiles: %w", err)
	}
	for _, player := range players {
		levels[player.UID] = player.Level
	}
	return levels, nil
}
