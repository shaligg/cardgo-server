// Package workshop 提供工坊总览、设施成长和离线收益相关业务。
//
// 当前实现包含总览读取、设施升级和离线收益领取。
package workshop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"gorm.io/gorm"
)

const (
	minOfflineRewardSeconds    int64 = 300
	maxOfflineRewardSeconds    int64 = 14400
	baseOfflineGoldPerHour     int64 = 20
	baseOfflineMaterialPerHour int64 = 1
)

var (
	// ErrFacilityNotFound 表示请求中的设施 ID 不在 MVP 设施列表中。
	ErrFacilityNotFound = errors.New("facility not found")
)

// Service 是工坊模块应用服务。
type Service struct {
	Repo        repo.WorkshopRepository
	Assets      asset.Service
	Tx          idb.TxManager
	PlayerCache repo.PlayerCacheInvalidator
	// Players 用于读取影响离线收益的玩家等级；为空时按 1 级兜底。
	Players repo.PlayerRepository
	Data    *gamedata.WorkshopData
	// Now 是可替换时间源，主要用于离线收益单元测试。
	Now func() time.Time
}

// OfflineRewardPreview 是离线收益预览。
//
// 总览和领取使用同一套计算规则，避免客户端预览和实际发奖不一致。
type OfflineRewardPreview struct {
	OfflineSeconds int64 `json:"offline_seconds"`
	Gold           int64 `json:"gold"`
	BasicMaterial  int64 `json:"basic_material"`
}

// Overview 是 workshop.get_overview 返回给客户端的总览数据。
type Overview struct {
	Workshop             repo.PlayerWorkshop   `json:"workshop"`
	Facilities           []repo.PlayerFacility `json:"facilities"`
	Decorations          []interface{}         `json:"decorations"`
	DisplayDecorations   []interface{}         `json:"display_decorations"`
	OfflineRewardPreview OfflineRewardPreview  `json:"offline_reward_preview"`
	UpgradeAvailable     bool                  `json:"upgrade_available"`
}

// FacilityUpgradeResult 是 workshop.upgrade_facility 的返回数据。
type FacilityUpgradeResult struct {
	Facility           repo.PlayerFacility `json:"facility"`
	FacilityID         string              `json:"facility_id"`
	OldLevel           int                 `json:"old_level"`
	NewLevel           int                 `json:"new_level"`
	GoldCost           int64               `json:"gold_cost"`
	Costs              []asset.CostItem    `json:"costs,omitempty"`
	Effects            []interface{}       `json:"effects"`
	NextUpgradePreview *UpgradePreview     `json:"next_upgrade_preview,omitempty"`
	Player             *repo.Player        `json:"player,omitempty"`
}

// UpgradePreview 是下一级升级预览。
type UpgradePreview struct {
	FromLevel int              `json:"from_level"`
	ToLevel   int              `json:"to_level"`
	GoldCost  int64            `json:"gold_cost"`
	Costs     []asset.CostItem `json:"costs,omitempty"`
}

// OfflineRewardClaimResult 是 workshop.claim_offline_reward 的返回数据。
type OfflineRewardClaimResult struct {
	UID              string               `json:"uid"`
	OfflineSeconds   int64                `json:"offline_seconds"`
	EffectiveSeconds int64                `json:"effective_seconds"`
	Gold             int64                `json:"gold"`
	Rewards          []asset.RewardItem   `json:"rewards"`
	Player           *repo.Player         `json:"player,omitempty"`
	ClaimedAt        int64                `json:"claimed_at"`
	Preview          OfflineRewardPreview `json:"preview"`
}

// GetOverview 获取玩家工坊总览。
//
// 新玩家第一次请求时会创建默认 PlayerWorkshop；设施记录在玩家首次升级对应设施时创建。
func (s Service) GetOverview(ctx context.Context, uid string) (Overview, error) {
	if s.Repo == nil {
		return Overview{}, fmt.Errorf("workshop repository is nil")
	}
	workshop, err := s.Repo.GetOrCreateWorkshop(ctx, uid)
	if err != nil {
		return Overview{}, err
	}
	facilities, err := s.Repo.GetFacilities(ctx, uid)
	if err != nil {
		return Overview{}, err
	}
	preview, err := s.buildOfflineRewardPreview(ctx, workshop)
	if err != nil {
		return Overview{}, err
	}
	return Overview{
		Workshop:             workshop,
		Facilities:           facilities,
		Decorations:          []interface{}{},
		DisplayDecorations:   []interface{}{},
		OfflineRewardPreview: preview,
		UpgradeAvailable:     false,
	}, nil
}

// UpgradeFacility 升级指定工坊设施。
//
// 扣费和设施升级由 Service 按配置编排到同一事务内完成。
func (s Service) UpgradeFacility(ctx context.Context, uid string, facilityID string, reqID string) (FacilityUpgradeResult, error) {
	if s.Repo == nil {
		return FacilityUpgradeResult{}, fmt.Errorf("workshop repository is nil")
	}
	if reqID == "" {
		return FacilityUpgradeResult{}, repo.ErrInvalidReqID
	}
	facilityConfig, ok := s.facilityConfig(facilityID)
	if !ok {
		return FacilityUpgradeResult{}, fmt.Errorf("%w: %s", ErrFacilityNotFound, facilityID)
	}
	current, err := s.Repo.GetOrCreateFacility(ctx, uid, facilityID)
	if err != nil {
		return FacilityUpgradeResult{}, err
	}
	if current.Level >= facilityConfig.MaxLevel {
		return FacilityUpgradeResult{}, repo.ErrFacilityMaxLevel
	}

	costs := s.upgradeCosts(facilityConfig, current.Level)
	var facility repo.PlayerFacility
	var player *repo.Player
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		results, err := s.Assets.ApplyCostInTx(ctx, tx, uid, costs, "workshop.upgrade_facility", reqID)
		if err != nil {
			return err
		}
		for _, result := range results {
			if result.Player != nil {
				player = result.Player
				break
			}
		}
		facility, err = s.Repo.UpgradeFacilityInTx(ctx, tx, uid, facilityID, facilityConfig.MaxLevel)
		return err
	}); err != nil {
		return FacilityUpgradeResult{}, err
	}
	if s.PlayerCache != nil {
		s.PlayerCache.InvalidatePlayer(uid)
	}
	result := s.buildUpgradeResult(facility, costs)
	result.Player = player
	return result, nil
}

// ClaimOfflineReward 领取当前累计的离线收益。
func (s Service) ClaimOfflineReward(ctx context.Context, uid string, reqID string) (OfflineRewardClaimResult, error) {
	if s.Repo == nil {
		return OfflineRewardClaimResult{}, fmt.Errorf("workshop repository is nil")
	}
	if reqID == "" {
		return OfflineRewardClaimResult{}, repo.ErrInvalidReqID
	}
	workshop, err := s.Repo.GetOrCreateWorkshop(ctx, uid)
	if err != nil {
		return OfflineRewardClaimResult{}, err
	}
	preview, err := s.buildOfflineRewardPreview(ctx, workshop)
	if err != nil {
		return OfflineRewardClaimResult{}, err
	}
	claim := repo.OfflineRewardClaim{
		UID:              uid,
		OfflineSeconds:   preview.OfflineSeconds,
		EffectiveSeconds: effectiveOfflineSeconds(preview.OfflineSeconds),
		Gold:             preview.Gold,
		BasicMaterial:    preview.BasicMaterial,
		ClaimedAt:        s.now().Unix(),
	}

	var player *repo.Player
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		rewards := offlineRewardItems(claim)
		if len(rewards) > 0 {
			results, err := s.Assets.ApplyRewardInTx(ctx, tx, uid, rewards, "workshop.claim_offline_reward", reqID)
			if err != nil {
				return err
			}
			for _, result := range results {
				if result.Player != nil {
					player = result.Player
					break
				}
			}
		}
		var err error
		claim, err = s.Repo.RecordOfflineRewardClaimInTx(ctx, tx, uid, claim)
		return err
	}); err != nil {
		return OfflineRewardClaimResult{}, err
	}
	if s.PlayerCache != nil && claim.Gold > 0 {
		s.PlayerCache.InvalidatePlayer(uid)
	}
	return s.buildClaimResult(ctx, claim, player)
}

func (s Service) buildUpgradeResult(facility repo.PlayerFacility, costs []asset.CostItem) FacilityUpgradeResult {
	result := FacilityUpgradeResult{
		Facility:   facility,
		FacilityID: facility.FacilityID,
		OldLevel:   facility.Level - 1,
		NewLevel:   facility.Level,
		GoldCost:   goldCostFromCosts(costs),
		Costs:      costs,
		Effects:    []interface{}{},
	}
	if facilityConfig, ok := s.facilityConfig(facility.FacilityID); ok && facility.Level < facilityConfig.MaxLevel {
		nextCosts := s.upgradeCosts(facilityConfig, facility.Level)
		result.NextUpgradePreview = &UpgradePreview{
			FromLevel: facility.Level,
			ToLevel:   facility.Level + 1,
			GoldCost:  goldCostFromCosts(nextCosts),
			Costs:     nextCosts,
		}
	}
	return result
}

func (s Service) buildOfflineRewardPreview(ctx context.Context, workshop repo.PlayerWorkshop) (OfflineRewardPreview, error) {
	offlineSeconds := s.now().Unix() - workshop.LastOfflineRewardAt
	if offlineSeconds < 0 {
		offlineSeconds = 0
	}
	preview := OfflineRewardPreview{OfflineSeconds: offlineSeconds}
	effectiveSeconds := effectiveOfflineSeconds(offlineSeconds)
	if effectiveSeconds <= 0 {
		return preview, nil
	}

	playerLevel := 1
	if s.Players != nil {
		player, err := s.Players.GetByUID(ctx, workshop.UID)
		if err != nil {
			return OfflineRewardPreview{}, err
		}
		playerLevel = player.Level
	}
	preview.Gold = int64(playerLevel) * baseOfflineGoldPerHour * effectiveSeconds / 3600
	preview.BasicMaterial = baseOfflineMaterialPerHour * effectiveSeconds / 3600
	return preview, nil
}

func (s Service) buildClaimResult(ctx context.Context, claim repo.OfflineRewardClaim, player *repo.Player) (OfflineRewardClaimResult, error) {
	if player == nil && claim.Gold > 0 && s.Players != nil {
		current, err := s.Players.GetByUID(ctx, claim.UID)
		if err != nil {
			return OfflineRewardClaimResult{}, err
		}
		player = &current
	}
	rewards := offlineRewardItems(claim)
	return OfflineRewardClaimResult{
		UID:              claim.UID,
		OfflineSeconds:   claim.OfflineSeconds,
		EffectiveSeconds: claim.EffectiveSeconds,
		Gold:             claim.Gold,
		Rewards:          rewards,
		Player:           player,
		ClaimedAt:        claim.ClaimedAt,
		Preview: OfflineRewardPreview{
			OfflineSeconds: claim.OfflineSeconds,
			Gold:           claim.Gold,
			BasicMaterial:  claim.BasicMaterial,
		},
	}, nil
}

func offlineRewardItems(claim repo.OfflineRewardClaim) []asset.RewardItem {
	rewards := []asset.RewardItem{}
	if claim.Gold > 0 {
		rewards = append(rewards, asset.RewardItem{ItemID: gamedata.ItemIDGold, Count: claim.Gold})
	}
	if claim.BasicMaterial > 0 {
		rewards = append(rewards, asset.RewardItem{ItemID: gamedata.ItemIDBasicMaterial, Count: claim.BasicMaterial})
	}
	return rewards
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func effectiveOfflineSeconds(offlineSeconds int64) int64 {
	if offlineSeconds < minOfflineRewardSeconds {
		return 0
	}
	if offlineSeconds > maxOfflineRewardSeconds {
		return maxOfflineRewardSeconds
	}
	return offlineSeconds
}

func (s Service) facilityConfig(facilityID string) (gamedata.FacilityConfig, bool) {
	if s.Data == nil {
		return gamedata.FacilityConfig{}, false
	}
	cfg, ok := s.Data.Facilities[facilityID]
	return cfg, ok
}

func (s Service) upgradeCosts(facility gamedata.FacilityConfig, currentLevel int) []asset.CostItem {
	targetLevel := currentLevel + 1
	for _, level := range facility.Levels {
		if level.Level != targetLevel {
			continue
		}
		costs := make([]asset.CostItem, 0, len(level.UpgradeCosts))
		for _, cost := range level.UpgradeCosts {
			costs = append(costs, asset.CostItem{ItemID: cost.ItemID, Count: cost.Count})
		}
		return costs
	}
	return nil
}

func goldCostFromCosts(costs []asset.CostItem) int64 {
	for _, cost := range costs {
		if cost.ItemID == gamedata.ItemIDGold {
			return cost.Count
		}
	}
	return 0
}
