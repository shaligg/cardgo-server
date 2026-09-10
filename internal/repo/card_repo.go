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

// DBCardRepository 是基于 GORM 的卡牌与卡组仓储。
type DBCardRepository struct {
	db *gorm.DB
}

// NewDBCardRepository 创建卡牌与卡组仓储。
func NewDBCardRepository(db *gorm.DB) *DBCardRepository {
	return &DBCardRepository{db: db}
}

// GetCards 查询玩家拥有的卡牌列表。
func (r *DBCardRepository) GetCards(ctx context.Context, uid string) ([]PlayerCard, error) {
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
func (r *DBCardRepository) GetDeck(ctx context.Context, uid string, deckID int32) (PlayerDeck, error) {
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

// EnsureDefaultCards 为新玩家补齐初始卡牌，重复调用不会增加卡牌数量。
func (r *DBCardRepository) EnsureDefaultCards(ctx context.Context, uid string, cardIDs []int64) error {
	if len(cardIDs) == 0 {
		return nil
	}
	rows := make([]model.PlayerCard, 0, len(cardIDs))
	for _, cardID := range cardIDs {
		rows = append(rows, model.PlayerCard{UID: uid, CardID: cardID, Level: 1, Count: 1})
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "uid"}, {Name: "card_id"}}, DoNothing: true,
	}).Create(&rows).Error
}

// SaveDeck 保存玩家卡组。
func (r *DBCardRepository) SaveDeck(ctx context.Context, uid string, deckID int32, name string, cardIDs []int64) (PlayerDeck, error) {
	cardIDsJSON, err := json.Marshal(cardIDs)
	if err != nil {
		return PlayerDeck{}, fmt.Errorf("marshal deck card_ids: %w", err)
	}
	row := model.PlayerDeck{UID: uid, DeckID: deckID, Name: name, CardIDsJSON: string(cardIDsJSON), IsActive: deckID == 1}
	if err := r.db.WithContext(ctx).Where("uid = ? AND deck_id = ?", uid, deckID).Assign(row).FirstOrCreate(&row).Error; err != nil {
		return PlayerDeck{}, fmt.Errorf("save player deck: %w", err)
	}
	return toDomainPlayerDeck(row)
}

// GetCardInTx 在外部事务中查询玩家指定卡牌。
func (r *DBCardRepository) GetCardInTx(ctx context.Context, tx *gorm.DB, uid string, cardID int64) (PlayerCard, error) {
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
	return toDomainPlayerCard(row), nil
}

// UpdateCardInTx 保存业务层已经计算完成的卡牌数据。
func (r *DBCardRepository) UpdateCardInTx(ctx context.Context, tx *gorm.DB, card PlayerCard) error {
	if tx == nil {
		return fmt.Errorf("transaction is nil")
	}
	result := tx.WithContext(ctx).Model(&model.PlayerCard{}).
		Where("uid = ? AND card_id = ?", card.UID, card.CardID).
		Updates(map[string]interface{}{"level": card.Level, "exp": card.Exp, "count": card.Count})
	if result.Error != nil {
		return fmt.Errorf("update player card: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCardNotOwned
	}
	return nil
}

func toDomainPlayerCard(row model.PlayerCard) PlayerCard {
	return PlayerCard{UID: row.UID, CardID: row.CardID, Level: row.Level, Exp: row.Exp, Count: row.Count}
}

func toDomainPlayerDeck(row model.PlayerDeck) (PlayerDeck, error) {
	var cardIDs []int64
	if row.CardIDsJSON != "" {
		if err := json.Unmarshal([]byte(row.CardIDsJSON), &cardIDs); err != nil {
			return PlayerDeck{}, fmt.Errorf("unmarshal deck card_ids: %w", err)
		}
	}
	return PlayerDeck{UID: row.UID, DeckID: row.DeckID, Name: row.Name, CardIDs: cardIDs, IsActive: row.IsActive}, nil
}
