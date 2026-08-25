package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBPlayerRepository 是基于 GORM 的玩家与背包仓储实现。
//
// 当前同时实现 PlayerRepository 和 InventoryRepository，后续如果表或库拆分，可以在不改业务层的前提下拆成多个实现。
type DBPlayerRepository struct {
	db *gorm.DB
}

// NewDBPlayerRepository 创建数据库仓储实例。
func NewDBPlayerRepository(db *gorm.DB) *DBPlayerRepository {
	return &DBPlayerRepository{db: db}
}

// Migrate 自动迁移当前 MVP 需要的数据库表。
//
// 正式生产环境可以替换为独立 migration 工具，但 demo 阶段保留这里能简化启动流程。
func (r *DBPlayerRepository) Migrate() error {
	return r.db.AutoMigrate(&model.Player{}, &model.InventoryItem{}, &model.PlayerCard{}, &model.PlayerDeck{}, &model.PlayerWorkshop{}, &model.PlayerFacility{}, &model.AssetLog{})
}

// GetByUID 查询玩家基础数据；玩家不存在时会创建默认数据。
func (r *DBPlayerRepository) GetByUID(ctx context.Context, uid string) (Player, error) {
	m, err := r.getOrCreate(ctx, r.db.WithContext(ctx), uid)
	if err != nil {
		return Player{}, err
	}
	return toDomainPlayer(m), nil
}

// ChangeGold 在事务中变更玩家金币并记录资产流水。
func (r *DBPlayerRepository) ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error) {
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
func (r *DBPlayerRepository) ChangeGoldInTx(ctx context.Context, tx *gorm.DB, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error) {
	if reqID == "" {
		return Player{}, ErrInvalidReqID
	}
	if tx == nil {
		return Player{}, fmt.Errorf("transaction is nil")
	}
	if reason == "" {
		reason = "asset.change_gold"
	}
	current, err := r.getOrCreate(ctx, tx, uid)
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

	if err := insertGoldAssetLog(tx.WithContext(ctx), uid, itemID, delta, current.Gold, reason, reqID); err != nil {
		return Player{}, err
	}
	return toDomainPlayer(current), nil
}

// GetInventory 查询玩家通用可堆叠背包。
func (r *DBPlayerRepository) GetInventory(ctx context.Context, uid string) ([]InventoryItem, error) {
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
func (r *DBPlayerRepository) ChangeInventoryItem(ctx context.Context, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error) {
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
func (r *DBPlayerRepository) ChangeInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error) {
	if reqID == "" {
		return InventoryItem{}, ErrInvalidReqID
	}
	if tx == nil {
		return InventoryItem{}, fmt.Errorf("transaction is nil")
	}
	if reason == "" {
		reason = "asset.change_item"
	}
	current, err := r.getOrCreateInventoryItem(ctx, tx, uid, itemID)
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

	if err := tx.WithContext(ctx).Create(&model.AssetLog{
		UID:     uid,
		ItemID:  itemID,
		Delta:   delta,
		Balance: current.Count,
		Reason:  reason,
		ReqID:   reqID,
	}).Error; err != nil {
		return InventoryItem{}, fmt.Errorf("insert asset log: %w", err)
	}
	return toDomainInventoryItem(current), nil
}

// GetCards 查询玩家拥有的卡牌列表。
func (r *DBPlayerRepository) GetCards(ctx context.Context, uid string) ([]PlayerCard, error) {
	var rows []model.PlayerCard
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).Order("card_id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query player cards: %w", err)
	}
	out := make([]PlayerCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomainPlayerCard(row))
	}
	return out, nil
}

// GetDeck 查询玩家指定卡组。
func (r *DBPlayerRepository) GetDeck(ctx context.Context, uid string, deckID int32) (PlayerDeck, error) {
	var row model.PlayerDeck
	err := r.db.WithContext(ctx).Where("uid = ? AND deck_id = ?", uid, deckID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerDeck{}, ErrDeckNotFound
	}
	if err != nil {
		return PlayerDeck{}, fmt.Errorf("query player deck: %w", err)
	}
	return toDomainPlayerDeck(row)
}

// EnsureDefaultCards 为新玩家补齐初始卡牌。
//
// 该方法是幂等的，重复调用不会增加卡牌数量。
func (r *DBPlayerRepository) EnsureDefaultCards(ctx context.Context, uid string, cardIDs []int64) error {
	if len(cardIDs) == 0 {
		return nil
	}
	rows := make([]model.PlayerCard, 0, len(cardIDs))
	for _, cardID := range cardIDs {
		rows = append(rows, model.PlayerCard{
			UID:    uid,
			CardID: cardID,
			Level:  1,
			Count:  1,
		})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}, {Name: "card_id"}},
		DoNothing: true,
	}).Create(&rows).Error
}

// SaveDeck 保存玩家卡组。
func (r *DBPlayerRepository) SaveDeck(ctx context.Context, uid string, deckID int32, name string, cardIDs []int64) (PlayerDeck, error) {
	cardIDsJSON, err := json.Marshal(cardIDs)
	if err != nil {
		return PlayerDeck{}, fmt.Errorf("marshal deck card_ids: %w", err)
	}
	row := model.PlayerDeck{
		UID:         uid,
		DeckID:      deckID,
		Name:        name,
		CardIDsJSON: string(cardIDsJSON),
		IsActive:    deckID == 1,
	}
	if err := r.db.WithContext(ctx).Where("uid = ? AND deck_id = ?", uid, deckID).Assign(row).FirstOrCreate(&row).Error; err != nil {
		return PlayerDeck{}, fmt.Errorf("save player deck: %w", err)
	}
	return toDomainPlayerDeck(row)
}

// UpgradeCard 在事务中提升玩家卡牌等级。
//
// 资产扣费由上层 CardService 通过 asset.Service 完成，本方法只处理卡牌数据。
func (r *DBPlayerRepository) UpgradeCard(ctx context.Context, uid string, cardID int64, maxLevel int) (PlayerCard, error) {
	var out PlayerCard
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = r.UpgradeCardInTx(ctx, tx, uid, cardID, maxLevel)
		return err
	})
	if err != nil {
		return PlayerCard{}, err
	}
	return out, nil
}

// UpgradeCardInTx 在外部事务中提升玩家卡牌等级。
func (r *DBPlayerRepository) UpgradeCardInTx(ctx context.Context, tx *gorm.DB, uid string, cardID int64, maxLevel int) (PlayerCard, error) {
	if tx == nil {
		return PlayerCard{}, fmt.Errorf("transaction is nil")
	}
	var row model.PlayerCard
	err := tx.WithContext(ctx).Where("uid = ? AND card_id = ?", uid, cardID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PlayerCard{}, ErrCardNotOwned
	}
	if err != nil {
		return PlayerCard{}, fmt.Errorf("query player card: %w", err)
	}
	if row.Level >= maxLevel {
		return PlayerCard{}, ErrCardMaxLevel
	}
	row.Level++
	if err := tx.WithContext(ctx).Save(&row).Error; err != nil {
		return PlayerCard{}, fmt.Errorf("save player card: %w", err)
	}
	return toDomainPlayerCard(row), nil
}

// getOrCreate 在当前事务中查询或创建玩家默认数据。
func (r *DBPlayerRepository) getOrCreate(ctx context.Context, tx *gorm.DB, uid string) (model.Player, error) {
	var m model.Player
	err := tx.WithContext(ctx).Where("uid = ?", uid).Take(&m).Error
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Player{}, fmt.Errorf("query player: %w", err)
	}

	m = model.Player{
		UID:   uid,
		Level: 1,
		Gold:  0,
	}
	if err := tx.WithContext(ctx).Create(&m).Error; err != nil {
		return model.Player{}, fmt.Errorf("create player: %w", err)
	}
	return m, nil
}

// getOrCreateInventoryItem 在当前事务中查询或创建背包道具行。
func (r *DBPlayerRepository) getOrCreateInventoryItem(ctx context.Context, tx *gorm.DB, uid string, itemID int64) (model.InventoryItem, error) {
	var m model.InventoryItem
	err := tx.WithContext(ctx).Where("uid = ? AND item_id = ?", uid, itemID).Take(&m).Error
	if err == nil {
		return m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.InventoryItem{}, fmt.Errorf("query inventory item: %w", err)
	}

	m = model.InventoryItem{UID: uid, ItemID: itemID, Count: 0}
	if err := tx.WithContext(ctx).Create(&m).Error; err != nil {
		return model.InventoryItem{}, fmt.Errorf("create inventory item: %w", err)
	}
	return m, nil
}

// toDomainInventoryItem 把数据库模型转换为业务层领域结构。
func toDomainInventoryItem(m model.InventoryItem) InventoryItem {
	return InventoryItem{UID: m.UID, ItemID: m.ItemID, Count: m.Count}
}

// toDomainPlayerCard 把卡牌数据库模型转换为业务层结构。
func toDomainPlayerCard(m model.PlayerCard) PlayerCard {
	return PlayerCard{
		UID:    m.UID,
		CardID: m.CardID,
		Level:  m.Level,
		Exp:    m.Exp,
		Count:  m.Count,
	}
}

// toDomainPlayerDeck 把卡组数据库模型转换为业务层结构。
func toDomainPlayerDeck(m model.PlayerDeck) (PlayerDeck, error) {
	var cardIDs []int64
	if m.CardIDsJSON != "" {
		if err := json.Unmarshal([]byte(m.CardIDsJSON), &cardIDs); err != nil {
			return PlayerDeck{}, fmt.Errorf("unmarshal deck card_ids: %w", err)
		}
	}
	return PlayerDeck{
		UID:      m.UID,
		DeckID:   m.DeckID,
		Name:     m.Name,
		CardIDs:  cardIDs,
		IsActive: m.IsActive,
	}, nil
}

// toDomainPlayer 把数据库模型转换为业务层领域结构。
func toDomainPlayer(m model.Player) Player {
	return Player{
		UID:   m.UID,
		Level: m.Level,
		Gold:  m.Gold,
	}
}

func (r *DBPlayerRepository) debitGoldInTx(ctx context.Context, tx *gorm.DB, uid string, amount int64) (model.Player, error) {
	current, err := r.getOrCreate(ctx, tx, uid)
	if err != nil {
		return model.Player{}, err
	}
	if current.Gold-amount < 0 {
		return model.Player{}, ErrInsufficientGold
	}
	current.Gold -= amount
	if err := tx.Save(&current).Error; err != nil {
		return model.Player{}, fmt.Errorf("save player: %w", err)
	}
	return current, nil
}

func (r *DBPlayerRepository) creditGoldInTx(ctx context.Context, tx *gorm.DB, uid string, amount int64) (model.Player, error) {
	if amount < 0 {
		return model.Player{}, ErrInvalidAmount
	}
	current, err := r.getOrCreate(ctx, tx, uid)
	if err != nil {
		return model.Player{}, err
	}
	current.Gold += amount
	if err := tx.Save(&current).Error; err != nil {
		return model.Player{}, fmt.Errorf("save player: %w", err)
	}
	return current, nil
}

func insertGoldAssetLog(tx *gorm.DB, uid string, itemID int64, delta int64, balance int64, reason string, reqID string) error {
	if err := tx.Create(&model.AssetLog{
		UID:     uid,
		ItemID:  itemID,
		Delta:   delta,
		Balance: balance,
		Reason:  reason,
		ReqID:   reqID,
	}).Error; err != nil {
		return fmt.Errorf("insert asset log: %w", err)
	}
	return nil
}
