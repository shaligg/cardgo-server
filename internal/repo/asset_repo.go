package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

// DBAssetRepository 是基于 GORM 的玩家资产仓储。
//
// 金币存放在玩家主表，通用可堆叠道具存放在背包表，所有变更统一写资产流水。
type DBAssetRepository struct {
	db *gorm.DB
}

// NewDBAssetRepository 创建玩家资产仓储。
func NewDBAssetRepository(db *gorm.DB) *DBAssetRepository {
	return &DBAssetRepository{db: db}
}

// ChangeGold 在事务中变更玩家金币并记录资产流水。
func (r *DBAssetRepository) ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error) {
	var out Player
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = r.ChangeGoldInTx(ctx, tx, uid, delta, itemID, reason, reqID)
		return err
	})
	if err != nil {
		return Player{}, err
	}
	return out, nil
}

// ChangeGoldInTx 在外部事务中变更玩家金币。
func (r *DBAssetRepository) ChangeGoldInTx(ctx context.Context, tx *gorm.DB, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error) {
	if reqID == "" {
		return Player{}, ErrInvalidReqID
	}
	if tx == nil {
		return Player{}, fmt.Errorf("transaction is nil")
	}
	if reason == "" {
		reason = "asset.change_gold"
	}
	current, err := getOrCreatePlayer(ctx, tx, uid)
	if err != nil {
		return Player{}, err
	}
	if current.Gold+delta < 0 {
		return Player{}, ErrInsufficientGold
	}
	current.Gold += delta
	if err := tx.WithContext(ctx).Save(&current).Error; err != nil {
		return Player{}, fmt.Errorf("save player: %w", err)
	}
	if err := insertAssetLog(tx.WithContext(ctx), uid, itemID, delta, current.Gold, reason, reqID); err != nil {
		return Player{}, err
	}
	return toDomainPlayer(current), nil
}

// GetInventory 查询玩家通用可堆叠背包。
func (r *DBAssetRepository) GetInventory(ctx context.Context, uid string) ([]InventoryItem, error) {
	var rows []model.InventoryItem
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).Order("item_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query inventory: %w", err)
	}
	out := make([]InventoryItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainInventoryItem(row))
	}
	return out, nil
}

// ChangeInventoryItem 在事务中变更通用可堆叠背包道具并记录资产流水。
func (r *DBAssetRepository) ChangeInventoryItem(ctx context.Context, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error) {
	var out InventoryItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = r.ChangeInventoryItemInTx(ctx, tx, uid, itemID, delta, reason, reqID)
		return err
	})
	if err != nil {
		return InventoryItem{}, err
	}
	return out, nil
}

// ChangeInventoryItemInTx 在外部事务中变更通用可堆叠背包道具。
func (r *DBAssetRepository) ChangeInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error) {
	if reqID == "" {
		return InventoryItem{}, ErrInvalidReqID
	}
	if tx == nil {
		return InventoryItem{}, fmt.Errorf("transaction is nil")
	}
	if reason == "" {
		reason = "asset.change_item"
	}
	current, err := getOrCreateInventoryItem(ctx, tx, uid, itemID)
	if err != nil {
		return InventoryItem{}, err
	}
	if current.Count+delta < 0 {
		return InventoryItem{}, ErrInsufficientItem
	}
	current.Count += delta
	if err := tx.WithContext(ctx).Save(&current).Error; err != nil {
		return InventoryItem{}, fmt.Errorf("save inventory item: %w", err)
	}
	if err := insertAssetLog(tx.WithContext(ctx), uid, itemID, delta, current.Count, reason, reqID); err != nil {
		return InventoryItem{}, err
	}
	return toDomainInventoryItem(current), nil
}

func getOrCreateInventoryItem(ctx context.Context, tx *gorm.DB, uid string, itemID int64) (model.InventoryItem, error) {
	var row model.InventoryItem
	err := tx.WithContext(ctx).Where("uid = ? AND item_id = ?", uid, itemID).Take(&row).Error
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.InventoryItem{}, fmt.Errorf("query inventory item: %w", err)
	}
	row = model.InventoryItem{UID: uid, ItemID: itemID, Count: 0}
	if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
		return model.InventoryItem{}, fmt.Errorf("create inventory item: %w", err)
	}
	return row, nil
}

func toDomainInventoryItem(row model.InventoryItem) InventoryItem {
	return InventoryItem{UID: row.UID, ItemID: row.ItemID, Count: row.Count}
}

func insertAssetLog(tx *gorm.DB, uid string, itemID int64, delta int64, balance int64, reason string, reqID string) error {
	if err := tx.Create(&model.AssetLog{
		UID: uid, ItemID: itemID, Delta: delta, Balance: balance, Reason: reason, ReqID: reqID,
	}).Error; err != nil {
		return fmt.Errorf("insert asset log: %w", err)
	}
	return nil
}
