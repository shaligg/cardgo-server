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

// GetPlayerAssetInTx 在指定事务中读取玩家字段类资产。
func (r *DBAssetRepository) GetPlayerAssetInTx(ctx context.Context, tx *gorm.DB, uid string) (Player, error) {
	if tx == nil {
		return Player{}, fmt.Errorf("transaction is nil")
	}
	current, err := getPlayer(ctx, tx, uid)
	if err != nil {
		return Player{}, err
	}
	return toDomainPlayer(current), nil
}

// SaveGoldInTx 保存业务层已经计算完成的金币余额，并写入同一事务的资产流水。
func (r *DBAssetRepository) SaveGoldInTx(ctx context.Context, tx *gorm.DB, uid string, balance int64, itemID int64, delta int64, reason string, reqID string) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	result := tx.WithContext(ctx).Model(&model.Player{}).Where("uid = ?", uid).Update("gold", balance)
	if result.Error != nil {
		return fmt.Errorf("update player gold: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPlayerNotFound
	}
	return insertAssetLog(tx.WithContext(ctx), uid, itemID, delta, balance, reason, reqID)
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

// GetOrCreateInventoryItemInTx 在指定事务中读取背包项，不存在时创建零值记录。
func (r *DBAssetRepository) GetOrCreateInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64) (InventoryItem, error) {
	if tx == nil {
		return InventoryItem{}, fmt.Errorf("transaction is nil")
	}
	current, err := getOrCreateInventoryItem(ctx, tx, uid, itemID)
	if err != nil {
		return InventoryItem{}, err
	}
	return toDomainInventoryItem(current), nil
}

// SaveInventoryItemInTx 保存业务层已经计算完成的背包数量，并写入同一事务的资产流水。
func (r *DBAssetRepository) SaveInventoryItemInTx(ctx context.Context, tx *gorm.DB, item InventoryItem, delta int64, reason string, reqID string) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	result := tx.WithContext(ctx).Model(&model.InventoryItem{}).
		Where("uid = ? AND item_id = ?", item.UID, item.ItemID).
		Update("count", item.Count)
	if result.Error != nil {
		return fmt.Errorf("update inventory item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("inventory item not found")
	}
	return insertAssetLog(tx.WithContext(ctx), item.UID, item.ItemID, delta, item.Count, reason, reqID)
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
