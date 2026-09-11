package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBWorkshopRepository 是基于 GORM 的工坊仓储。
type DBWorkshopRepository struct {
	db *gorm.DB
}

// NewDBWorkshopRepository 创建工坊仓储。
func NewDBWorkshopRepository(db *gorm.DB) *DBWorkshopRepository {
	return &DBWorkshopRepository{db: db}
}

// GetWorkshop 查询玩家工坊基础数据，不在读路径隐式创建默认数据。
func (r *DBWorkshopRepository) GetWorkshop(ctx context.Context, uid string) (PlayerWorkshop, error) {
	var row model.PlayerWorkshop
	err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
	if err == nil {
		return toDomainPlayerWorkshop(row), nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerWorkshop{}, ErrPlayerWorkshopNotFound
	}
	return PlayerWorkshop{}, fmt.Errorf("query player workshop: %w", err)
}

// CreateWorkshopIfAbsent 幂等创建业务层给出的默认工坊，并返回最终数据。
func (r *DBWorkshopRepository) CreateWorkshopIfAbsent(ctx context.Context, workshop PlayerWorkshop) (PlayerWorkshop, error) {
	row := model.PlayerWorkshop{
		UID:                 workshop.UID,
		Level:               workshop.Level,
		ActiveThemeID:       workshop.ActiveThemeID,
		LastOfflineRewardAt: time.Unix(workshop.LastOfflineRewardAt, 0),
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return PlayerWorkshop{}, fmt.Errorf("create player workshop: %w", err)
	}
	return r.GetWorkshop(ctx, workshop.UID)
}

// GetFacilities 查询玩家已有设施数据。
func (r *DBWorkshopRepository) GetFacilities(ctx context.Context, uid string) ([]PlayerFacility, error) {
	var rows []model.PlayerFacility
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).Order("facility_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query player facilities: %w", err)
	}
	out := make([]PlayerFacility, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainPlayerFacility(row))
	}
	return out, nil
}

// GetFacilityInTx 在外部事务中查询玩家指定设施。
func (r *DBWorkshopRepository) GetFacilityInTx(ctx context.Context, tx *gorm.DB, uid string, facilityID string) (PlayerFacility, error) {
	if tx == nil {
		return PlayerFacility{}, fmt.Errorf("transaction is nil")
	}
	var row model.PlayerFacility
	err := tx.WithContext(ctx).Where("uid = ? AND facility_id = ?", uid, facilityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerFacility{}, ErrPlayerFacilityNotFound
	}
	if err != nil {
		return PlayerFacility{}, fmt.Errorf("query player facility: %w", err)
	}
	return toDomainPlayerFacility(row), nil
}

// CreateFacilityInTx 创建业务层已经计算完成的设施数据。
func (r *DBWorkshopRepository) CreateFacilityInTx(ctx context.Context, tx *gorm.DB, facility PlayerFacility) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	row, err := playerFacilityModel(facility)
	if err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create player facility: %w", err)
	}
	return nil
}

// UpdateFacilityInTx 保存业务层已经计算完成的设施数据。
func (r *DBWorkshopRepository) UpdateFacilityInTx(ctx context.Context, tx *gorm.DB, facility PlayerFacility) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	row, err := playerFacilityModel(facility)
	if err != nil {
		return err
	}
	result := tx.WithContext(ctx).
		Model(&model.PlayerFacility{}).
		Where("uid = ? AND facility_id = ?", facility.UID, facility.FacilityID).
		Updates(map[string]interface{}{
			"level":       row.Level,
			"unlocked":    row.Unlocked,
			"unlocked_at": row.UnlockedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update player facility: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPlayerFacilityNotFound
	}
	return nil
}

func playerFacilityModel(facility PlayerFacility) (model.PlayerFacility, error) {
	if facility.UID == "" || facility.FacilityID == "" || facility.Level <= 0 {
		return model.PlayerFacility{}, fmt.Errorf("invalid player facility uid=%s facility_id=%s level=%d", facility.UID, facility.FacilityID, facility.Level)
	}
	row := model.PlayerFacility{
		UID:        facility.UID,
		FacilityID: facility.FacilityID,
		Level:      facility.Level,
		Unlocked:   facility.Unlocked,
	}
	if facility.UnlockedAt > 0 {
		unlockedAt := time.Unix(facility.UnlockedAt, 0)
		row.UnlockedAt = &unlockedAt
	}
	return row, nil
}

// UpdateLastOfflineRewardAtInTx 保存业务层已经决定推进的离线收益领取时间。
func (r *DBWorkshopRepository) UpdateLastOfflineRewardAtInTx(ctx context.Context, tx *gorm.DB, uid string, claimedAt int64) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	result := tx.WithContext(ctx).Model(&model.PlayerWorkshop{}).
		Where("uid = ?", uid).
		Update("last_offline_reward_at", time.Unix(claimedAt, 0))
	if result.Error != nil {
		return fmt.Errorf("update player workshop offline reward time: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPlayerWorkshopNotFound
	}
	return nil
}

func toDomainPlayerWorkshop(m model.PlayerWorkshop) PlayerWorkshop {
	return PlayerWorkshop{
		UID:                 m.UID,
		Level:               m.Level,
		ActiveThemeID:       m.ActiveThemeID,
		LastOfflineRewardAt: m.LastOfflineRewardAt.Unix(),
	}
}

func toDomainPlayerFacility(m model.PlayerFacility) PlayerFacility {
	out := PlayerFacility{
		UID:        m.UID,
		FacilityID: m.FacilityID,
		Level:      m.Level,
		Unlocked:   m.Unlocked,
	}
	if m.UnlockedAt != nil {
		out.UnlockedAt = m.UnlockedAt.Unix()
	}
	return out
}
