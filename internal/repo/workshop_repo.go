package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

const defaultWorkshopThemeID = "default"

// GetOrCreateWorkshop 查询玩家工坊基础数据；不存在时创建默认工坊。
func (r *DBPlayerRepository) GetOrCreateWorkshop(ctx context.Context, uid string) (PlayerWorkshop, error) {
	var row model.PlayerWorkshop
	err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
	if err == nil {
		return toDomainPlayerWorkshop(row), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerWorkshop{}, fmt.Errorf("query player workshop: %w", err)
	}

	now := time.Now()
	row = model.PlayerWorkshop{
		UID:                 uid,
		Level:               1,
		ActiveThemeID:       defaultWorkshopThemeID,
		LastOfflineRewardAt: now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return PlayerWorkshop{}, fmt.Errorf("create player workshop: %w", err)
	}
	return toDomainPlayerWorkshop(row), nil
}

// GetFacilities 查询玩家已有设施数据。
func (r *DBPlayerRepository) GetFacilities(ctx context.Context, uid string) ([]PlayerFacility, error) {
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

// GetOrCreateFacility 查询玩家设施；不存在时创建 Lv.1 已解锁默认设施。
func (r *DBPlayerRepository) GetOrCreateFacility(ctx context.Context, uid string, facilityID string) (PlayerFacility, error) {
	var row model.PlayerFacility
	err := r.db.WithContext(ctx).Where("uid = ? AND facility_id = ?", uid, facilityID).Take(&row).Error
	if err == nil {
		return toDomainPlayerFacility(row), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerFacility{}, fmt.Errorf("query player facility: %w", err)
	}

	now := time.Now()
	row = model.PlayerFacility{
		UID:        uid,
		FacilityID: facilityID,
		Level:      1,
		Unlocked:   true,
		UnlockedAt: &now,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return PlayerFacility{}, fmt.Errorf("create player facility: %w", err)
	}
	return toDomainPlayerFacility(row), nil
}

// GetFacilityUpgradeResult 查询设施升级请求是否已经执行过。
func (r *DBPlayerRepository) GetFacilityUpgradeResult(ctx context.Context, uid string, facilityID string, reqID string) (PlayerFacility, bool, error) {
	if reqID == "" {
		return PlayerFacility{}, false, ErrInvalidReqID
	}
	var out PlayerFacility
	handled, err := loadIdempotencyResult(r.db.WithContext(ctx), uid, facilityUpgradeAction(facilityID), reqID, &out)
	if err != nil {
		return PlayerFacility{}, false, err
	}
	return out, handled, nil
}

// UpgradeFacility 在事务中提升设施等级，并保证 reqID 幂等。
func (r *DBPlayerRepository) UpgradeFacility(ctx context.Context, uid string, facilityID string, maxLevel int, reqID string) (PlayerFacility, error) {
	var out PlayerFacility
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = r.UpgradeFacilityInTx(ctx, tx, uid, facilityID, maxLevel, reqID)
		return err
	})
	if err != nil {
		return PlayerFacility{}, err
	}
	return out, nil
}

// UpgradeFacilityInTx 在外部事务中提升设施等级，并保证 reqID 幂等。
func (r *DBPlayerRepository) UpgradeFacilityInTx(ctx context.Context, tx *gorm.DB, uid string, facilityID string, maxLevel int, reqID string) (PlayerFacility, error) {
	if reqID == "" {
		return PlayerFacility{}, ErrInvalidReqID
	}
	if tx == nil {
		return PlayerFacility{}, fmt.Errorf("transaction is nil")
	}
	action := facilityUpgradeAction(facilityID)

	var out PlayerFacility
	if handled, err := loadIdempotencyResult(tx.WithContext(ctx), uid, action, reqID, &out); handled || err != nil {
		return out, err
	}

	var row model.PlayerFacility
	err := tx.WithContext(ctx).Where("uid = ? AND facility_id = ?", uid, facilityID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		row = model.PlayerFacility{
			UID:        uid,
			FacilityID: facilityID,
			Level:      1,
			Unlocked:   true,
			UnlockedAt: &now,
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return PlayerFacility{}, fmt.Errorf("create player facility: %w", err)
		}
	} else if err != nil {
		return PlayerFacility{}, fmt.Errorf("query player facility: %w", err)
	}
	if row.Level >= maxLevel {
		return PlayerFacility{}, ErrFacilityMaxLevel
	}
	row.Level++
	if err := tx.WithContext(ctx).Save(&row).Error; err != nil {
		return PlayerFacility{}, fmt.Errorf("save player facility: %w", err)
	}
	out = toDomainPlayerFacility(row)
	if err := insertIdempotencyResult(tx.WithContext(ctx), uid, action, reqID, out); err != nil {
		return PlayerFacility{}, err
	}
	return out, nil
}

// GetOfflineRewardClaimResult 查询离线收益领取请求是否已经执行过。
func (r *DBPlayerRepository) GetOfflineRewardClaimResult(ctx context.Context, uid string, reqID string) (OfflineRewardClaim, bool, error) {
	if reqID == "" {
		return OfflineRewardClaim{}, false, ErrInvalidReqID
	}
	var out OfflineRewardClaim
	handled, err := loadIdempotencyResult(r.db.WithContext(ctx), uid, offlineRewardClaimAction(), reqID, &out)
	if err != nil {
		return OfflineRewardClaim{}, false, err
	}
	return out, handled, nil
}

// RecordOfflineRewardClaim 记录离线收益领取结果，并在有可结算时推进结算时间。
func (r *DBPlayerRepository) RecordOfflineRewardClaim(ctx context.Context, uid string, claim OfflineRewardClaim, reqID string) (OfflineRewardClaim, error) {
	var out OfflineRewardClaim
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = r.RecordOfflineRewardClaimInTx(ctx, tx, uid, claim, reqID)
		return err
	})
	if err != nil {
		return OfflineRewardClaim{}, err
	}
	return out, nil
}

// RecordOfflineRewardClaimInTx 在外部事务中记录离线收益领取结果。
func (r *DBPlayerRepository) RecordOfflineRewardClaimInTx(ctx context.Context, tx *gorm.DB, uid string, claim OfflineRewardClaim, reqID string) (OfflineRewardClaim, error) {
	if reqID == "" {
		return OfflineRewardClaim{}, ErrInvalidReqID
	}
	if tx == nil {
		return OfflineRewardClaim{}, fmt.Errorf("transaction is nil")
	}
	action := offlineRewardClaimAction()

	var out OfflineRewardClaim
	if handled, err := loadIdempotencyResult(tx.WithContext(ctx), uid, action, reqID, &out); handled || err != nil {
		return out, err
	}

	if claim.EffectiveSeconds > 0 {
		var row model.PlayerWorkshop
		err := tx.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = model.PlayerWorkshop{
				UID:                 uid,
				Level:               1,
				ActiveThemeID:       defaultWorkshopThemeID,
				LastOfflineRewardAt: time.Unix(claim.ClaimedAt, 0),
			}
			if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
				return OfflineRewardClaim{}, fmt.Errorf("create player workshop: %w", err)
			}
		} else if err != nil {
			return OfflineRewardClaim{}, fmt.Errorf("query player workshop: %w", err)
		} else {
			row.LastOfflineRewardAt = time.Unix(claim.ClaimedAt, 0)
			if err := tx.WithContext(ctx).Save(&row).Error; err != nil {
				return OfflineRewardClaim{}, fmt.Errorf("save player workshop: %w", err)
			}
		}
	}

	out = claim
	if err := insertIdempotencyResult(tx.WithContext(ctx), uid, action, reqID, out); err != nil {
		return OfflineRewardClaim{}, err
	}
	return out, nil
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

func facilityUpgradeAction(facilityID string) string {
	return fmt.Sprintf("workshop.upgrade_facility:%s", facilityID)
}

func offlineRewardClaimAction() string {
	return "workshop.claim_offline_reward"
}
