// Package asset 提供玩家资产变更的统一入口。
//
// 业务系统应通过这里发奖和扣费，由本模块根据 ItemConfig 路由到玩家表或背包表。
package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"gorm.io/gorm"
)

var (
	// ErrUnsupportedItemID 表示请求中的 item_id 没有在道具配置中定义。
	ErrUnsupportedItemID = errors.New("unsupported item_id")
	// ErrUnsupportedStorage 表示道具配置的存储方式当前资产模块不能处理。
	ErrUnsupportedStorage = errors.New("unsupported item storage")
	// ErrBatchNotSupported 表示某类资产存储暂不支持本次批量变更。
	ErrBatchNotSupported = errors.New("asset batch change is not supported yet")
	// ErrInsufficientGold 表示玩家金币不足。
	ErrInsufficientGold = errors.New("insufficient gold")
	// ErrInsufficientItem 表示玩家背包道具数量不足。
	ErrInsufficientItem = errors.New("insufficient item")
)

// RewardItem 表示一次发奖中的道具数量。
type RewardItem struct {
	ItemID int64 `json:"item_id"`
	Count  int64 `json:"count"`
}

// CostItem 表示一次扣费中的道具数量。
type CostItem struct {
	ItemID int64 `json:"item_id"`
	Count  int64 `json:"count"`
}

// ChangeResult 是资产变更后的返回结果。
//
// 玩家表字段类资产会返回 Player，背包可堆叠资产会返回 Item。
type ChangeResult struct {
	Player *repo.Player        `json:"player,omitempty"`
	Item   *repo.InventoryItem `json:"item,omitempty"`
}

// Service 是资产模块的应用服务。
//
// 它不直接访问数据库模型，而是依赖 repo 接口和道具配置完成路由、余额计算和事务编排。
type Service struct {
	Items         gamedata.ItemCatalog
	PlayerRepo    repo.PlayerAssetRepository
	InventoryRepo repo.InventoryAssetRepository
	Tx            idb.TransactionRunner
}

// Grant 发放奖励资产。
//
// reqID 必须由调用方提供，用于资产流水审计；普通请求重试由协议入口统一处理。
func (s Service) Grant(ctx context.Context, uid string, rewards []RewardItem, reason string, reqID string) ([]ChangeResult, error) {
	if reqID == "" {
		return nil, repo.ErrInvalidReqID
	}
	if len(rewards) == 0 {
		return nil, repo.ErrInvalidAmount
	}
	if s.Tx == nil {
		return nil, fmt.Errorf("transaction manager is nil")
	}

	var out []ChangeResult
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		var err error
		out, err = s.ApplyRewardInTx(ctx, tx, uid, rewards, reason, reqID)
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// Consume 扣除消耗资产。
//
// 单项和多项扣费都由 AssetService 在事务中执行。
func (s Service) Consume(ctx context.Context, uid string, costs []CostItem, reason string, reqID string) ([]ChangeResult, error) {
	if reqID == "" {
		return nil, repo.ErrInvalidReqID
	}
	if len(costs) == 0 {
		return nil, repo.ErrInvalidAmount
	}
	if s.Tx == nil {
		return nil, fmt.Errorf("transaction manager is nil")
	}

	var out []ChangeResult
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		var err error
		out, err = s.ApplyCostInTx(ctx, tx, uid, costs, reason, reqID)
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyRewardInTx 在外部事务中发放奖励资产。
//
// 本方法只负责按 reward_list 落库，不判断玩法是否可领奖，也不更新玩法状态。
func (s Service) ApplyRewardInTx(ctx context.Context, tx *gorm.DB, uid string, rewards []RewardItem, reason string, reqID string) ([]ChangeResult, error) {
	if reqID == "" {
		return nil, repo.ErrInvalidReqID
	}
	if len(rewards) == 0 {
		return nil, repo.ErrInvalidAmount
	}
	rewardList, err := mergeRewardItems(rewards)
	if err != nil {
		return nil, err
	}
	out := make([]ChangeResult, 0, len(rewardList))
	for _, reward := range rewardList {
		result, err := s.changeInTx(ctx, tx, uid, reward.ItemID, reward.Count, reason, reqID)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, nil
}

// ApplyCostInTx 在外部事务中扣除消耗资产。
//
// 本方法只负责按 cost_list 扣资产并保证不为负数，不判断玩法条件。
func (s Service) ApplyCostInTx(ctx context.Context, tx *gorm.DB, uid string, costs []CostItem, reason string, reqID string) ([]ChangeResult, error) {
	if reqID == "" {
		return nil, repo.ErrInvalidReqID
	}
	if len(costs) == 0 {
		return nil, repo.ErrInvalidAmount
	}
	costList, err := mergeCostItems(costs)
	if err != nil {
		return nil, err
	}
	out := make([]ChangeResult, 0, len(costList))
	for _, cost := range costList {
		result, err := s.changeInTx(ctx, tx, uid, cost.ItemID, -cost.Count, reason, reqID)
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, nil
}

// mergeRewardItems 在资产入账前合并相同 item_id，避免玩法层重复处理 coin:1 + coin:1 这类通用规则。
func mergeRewardItems(rewards []RewardItem) ([]RewardItem, error) {
	merged := make([]RewardItem, 0, len(rewards))
	index := map[int64]int{}
	for _, reward := range rewards {
		if reward.Count <= 0 {
			return nil, repo.ErrInvalidAmount
		}
		if pos, ok := index[reward.ItemID]; ok {
			merged[pos].Count += reward.Count
			continue
		}
		index[reward.ItemID] = len(merged)
		merged = append(merged, reward)
	}
	return merged, nil
}

// mergeCostItems 在资产扣费前合并相同 item_id，统一处理多处扣同一种资产的情况。
func mergeCostItems(costs []CostItem) ([]CostItem, error) {
	merged := make([]CostItem, 0, len(costs))
	index := map[int64]int{}
	for _, cost := range costs {
		if cost.Count <= 0 {
			return nil, repo.ErrInvalidAmount
		}
		if pos, ok := index[cost.ItemID]; ok {
			merged[pos].Count += cost.Count
			continue
		}
		index[cost.ItemID] = len(merged)
		merged = append(merged, cost)
	}
	return merged, nil
}

// changeInTx 根据道具配置在外部事务中把资产变更路由到具体存储。
func (s Service) changeInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64, delta int64, reason string, reqID string) (ChangeResult, error) {
	if tx == nil {
		return ChangeResult{}, fmt.Errorf("transaction is nil")
	}
	if s.Items == nil {
		return ChangeResult{}, fmt.Errorf("item catalog is nil")
	}
	item, ok := s.Items.GetItem(itemID)
	if !ok {
		return ChangeResult{}, fmt.Errorf("%w: %d", ErrUnsupportedItemID, itemID)
	}

	switch item.StorageType {
	case gamedata.StoragePlayerField:
		return s.changePlayerFieldInTx(ctx, tx, uid, item, delta, reason, reqID)
	case gamedata.StorageInventoryStack:
		return s.changeInventoryStackInTx(ctx, tx, uid, item, delta, reason, reqID)
	default:
		return ChangeResult{}, fmt.Errorf("%w: %d uses %s", ErrUnsupportedStorage, itemID, item.StorageType)
	}
}

func (s Service) changePlayerFieldInTx(ctx context.Context, tx *gorm.DB, uid string, item gamedata.ItemConfig, delta int64, reason string, reqID string) (ChangeResult, error) {
	if item.StorageKey != "gold" {
		return ChangeResult{}, fmt.Errorf("%w: unsupported player_field %q", ErrUnsupportedStorage, item.StorageKey)
	}
	if s.PlayerRepo == nil {
		return ChangeResult{}, fmt.Errorf("tx player repository is nil")
	}
	p, err := s.PlayerRepo.GetPlayerAssetInTx(ctx, tx, uid)
	if err != nil {
		return ChangeResult{}, err
	}
	if p.Gold+delta < 0 {
		return ChangeResult{}, ErrInsufficientGold
	}
	p.Gold += delta
	if reason == "" {
		reason = "asset.change_gold"
	}
	if err := s.PlayerRepo.SaveGoldInTx(ctx, tx, uid, p.Gold, item.ItemID, delta, reason, reqID); err != nil {
		return ChangeResult{}, err
	}
	return ChangeResult{Player: &p}, nil
}

func (s Service) changeInventoryStackInTx(ctx context.Context, tx *gorm.DB, uid string, item gamedata.ItemConfig, delta int64, reason string, reqID string) (ChangeResult, error) {
	if s.InventoryRepo == nil {
		return ChangeResult{}, fmt.Errorf("tx inventory repository is nil")
	}
	invItem, err := s.InventoryRepo.GetOrCreateInventoryItemInTx(ctx, tx, uid, item.ItemID)
	if err != nil {
		return ChangeResult{}, err
	}
	if invItem.Count+delta < 0 {
		return ChangeResult{}, ErrInsufficientItem
	}
	invItem.Count += delta
	if reason == "" {
		reason = "asset.change_item"
	}
	if err := s.InventoryRepo.SaveInventoryItemInTx(ctx, tx, invItem, delta, reason, reqID); err != nil {
		return ChangeResult{}, err
	}
	return ChangeResult{Item: &invItem}, nil
}
