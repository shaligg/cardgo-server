// Package card 提供玩家卡牌库存、卡组编辑和卡牌成长规则。
//
// 协议解析在 app/card_handler.go，本包只关心业务校验与调用 repo/asset。
package card

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"gorm.io/gorm"
)

const (
	DefaultDeckID   int32 = 1
	DefaultDeckSize       = 5
	MaxCardLevel          = 5
)

var (
	// ErrGameDataMissing 表示卡牌业务依赖的策划配置没有加载。
	ErrGameDataMissing = errors.New("game data is missing")
	// ErrCardNotFound 表示请求中的卡牌配置不存在。
	ErrCardNotFound = errors.New("card not found")
	// ErrInvalidDeck 表示卡组数量、重复卡或未拥有卡牌不合法。
	ErrInvalidDeck = errors.New("invalid deck")
)

// Service 是卡牌模块应用服务。
//
// MVP 升级消耗金币；扣费和卡牌升级由 Service 编排到同一事务内完成。
type Service struct {
	Repo        repo.CardRepository
	Assets      asset.Service
	Tx          idb.TxManager
	Data        *gamedata.GameData
	PlayerCache repo.PlayerCacheInvalidator
}

// CardsResult 是 card.get_cards 的返回数据。
type CardsResult struct {
	Cards           []repo.PlayerCard `json:"cards"`
	Deck            *repo.PlayerDeck  `json:"deck,omitempty"`
	DefaultDeckSize int               `json:"default_deck_size"`
	MaxCardLevel    int               `json:"max_card_level"`
}

// UpgradeResult 是 card.upgrade 的返回数据。
type UpgradeResult struct {
	Card     repo.PlayerCard  `json:"card"`
	GoldCost int64            `json:"gold_cost"`
	Costs    []asset.CostItem `json:"costs,omitempty"`
	Player   *repo.Player     `json:"player,omitempty"`
}

// GetCards 查询玩家卡牌，并为新玩家补齐初始卡。
func (s Service) GetCards(ctx context.Context, uid string) (CardsResult, error) {
	if err := s.ensureReady(); err != nil {
		return CardsResult{}, err
	}
	if err := s.Repo.EnsureDefaultCards(ctx, uid, s.defaultCardIDs()); err != nil {
		return CardsResult{}, err
	}
	cards, err := s.Repo.GetCards(ctx, uid)
	if err != nil {
		return CardsResult{}, err
	}
	result := CardsResult{
		Cards:           cards,
		DefaultDeckSize: DefaultDeckSize,
		MaxCardLevel:    MaxCardLevel,
	}
	if deck, err := s.Repo.GetDeck(ctx, uid, DefaultDeckID); err == nil {
		result.Deck = &deck
	} else if !errors.Is(err, repo.ErrDeckNotFound) {
		return CardsResult{}, err
	}
	return result, nil
}

// SaveDeck 保存玩家卡组。
//
// 这里校验卡组数量、重复卡、卡牌配置存在以及玩家是否拥有，避免非法卡组进入战斗。
func (s Service) SaveDeck(ctx context.Context, uid string, deckID int32, name string, cardIDs []int64, reqID string) (repo.PlayerDeck, error) {
	if err := s.ensureReady(); err != nil {
		return repo.PlayerDeck{}, err
	}
	if deckID == 0 {
		deckID = DefaultDeckID
	}
	if len(cardIDs) == 0 || len(cardIDs) > DefaultDeckSize {
		return repo.PlayerDeck{}, fmt.Errorf("%w: card count must be 1-%d", ErrInvalidDeck, DefaultDeckSize)
	}
	if err := s.Repo.EnsureDefaultCards(ctx, uid, s.defaultCardIDs()); err != nil {
		return repo.PlayerDeck{}, err
	}
	if err := s.validateOwnedCards(ctx, uid, cardIDs); err != nil {
		return repo.PlayerDeck{}, err
	}
	return s.Repo.SaveDeck(ctx, uid, deckID, name, cardIDs, reqID)
}

// UpgradeCard 提升卡牌等级。
//
// 升级前先做配置和拥有校验，再由 Service 在同一事务内完成扣金币、写流水、升级和幂等。
func (s Service) UpgradeCard(ctx context.Context, uid string, cardID int64, reqID string) (UpgradeResult, error) {
	if err := s.ensureReady(); err != nil {
		return UpgradeResult{}, err
	}
	if reqID == "" {
		return UpgradeResult{}, repo.ErrInvalidReqID
	}
	if card, handled, err := s.Repo.GetCardUpgradeResult(ctx, uid, cardID, reqID); err != nil {
		return UpgradeResult{}, err
	} else if handled {
		costs := s.upgradeCosts(s.Data.Cards[cardID], card.Level-1)
		return UpgradeResult{Card: card, GoldCost: goldCostFromCosts(costs), Costs: costs}, nil
	}
	cardConfig, ok := s.Data.Cards[cardID]
	if !ok {
		return UpgradeResult{}, fmt.Errorf("%w: %d", ErrCardNotFound, cardID)
	}
	if err := s.Repo.EnsureDefaultCards(ctx, uid, s.defaultCardIDs()); err != nil {
		return UpgradeResult{}, err
	}
	current, err := s.findOwnedCard(ctx, uid, cardID)
	if err != nil {
		return UpgradeResult{}, err
	}
	if current.Level >= MaxCardLevel {
		return UpgradeResult{}, repo.ErrCardMaxLevel
	}

	costs := s.upgradeCosts(cardConfig, current.Level)
	var card repo.PlayerCard
	var player *repo.Player
	if err := s.Tx.Do(ctx, func(tx *gorm.DB) error {
		results, err := s.Assets.ApplyCostInTx(ctx, tx, uid, costs, "card.upgrade", reqID)
		if err != nil {
			return err
		}
		for _, result := range results {
			if result.Player != nil {
				player = result.Player
				break
			}
		}
		card, err = s.Repo.UpgradeCardInTx(ctx, tx, uid, cardID, MaxCardLevel, reqID)
		return err
	}); err != nil {
		return UpgradeResult{}, err
	}
	if s.PlayerCache != nil {
		s.PlayerCache.InvalidatePlayer(uid)
	}
	return UpgradeResult{Card: card, GoldCost: goldCostFromCosts(costs), Costs: costs, Player: player}, nil
}

func (s Service) ensureReady() error {
	if s.Data == nil || len(s.Data.Cards) == 0 {
		return ErrGameDataMissing
	}
	if s.Repo == nil {
		return fmt.Errorf("card repository is nil")
	}
	return nil
}

func (s Service) validateOwnedCards(ctx context.Context, uid string, cardIDs []int64) error {
	owned := map[int64]bool{}
	cards, err := s.Repo.GetCards(ctx, uid)
	if err != nil {
		return err
	}
	for _, card := range cards {
		if card.Count > 0 {
			owned[card.CardID] = true
		}
	}
	seen := map[int64]bool{}
	for _, cardID := range cardIDs {
		if _, ok := s.Data.Cards[cardID]; !ok {
			return fmt.Errorf("%w: %d", ErrCardNotFound, cardID)
		}
		if seen[cardID] {
			return fmt.Errorf("%w: duplicated card_id %d", ErrInvalidDeck, cardID)
		}
		if !owned[cardID] {
			return fmt.Errorf("%w: %d", repo.ErrCardNotOwned, cardID)
		}
		seen[cardID] = true
	}
	return nil
}

func (s Service) findOwnedCard(ctx context.Context, uid string, cardID int64) (repo.PlayerCard, error) {
	cards, err := s.Repo.GetCards(ctx, uid)
	if err != nil {
		return repo.PlayerCard{}, err
	}
	for _, card := range cards {
		if card.CardID == cardID && card.Count > 0 {
			return card, nil
		}
	}
	return repo.PlayerCard{}, repo.ErrCardNotOwned
}

func (s Service) defaultCardIDs() []int64 {
	ids := make([]int64, 0, len(s.Data.Cards))
	for cardID := range s.Data.Cards {
		ids = append(ids, cardID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > DefaultDeckSize {
		ids = ids[:DefaultDeckSize]
	}
	return ids
}

func upgradeGoldCost(currentLevel int) int64 {
	return int64(currentLevel * 50)
}

func (s Service) upgradeCosts(card gamedata.CardConfig, currentLevel int) []asset.CostItem {
	targetLevel := currentLevel + 1
	for _, configured := range card.UpgradeCosts {
		if configured.TargetLevel != targetLevel {
			continue
		}
		costs := make([]asset.CostItem, 0, len(configured.Costs))
		for _, cost := range configured.Costs {
			costs = append(costs, asset.CostItem{ItemID: cost.ItemID, Count: cost.Count})
		}
		return costs
	}
	return []asset.CostItem{{ItemID: gamedata.ItemIDGold, Count: upgradeGoldCost(currentLevel)}}
}

func goldCostFromCosts(costs []asset.CostItem) int64 {
	for _, cost := range costs {
		if cost.ItemID == gamedata.ItemIDGold {
			return cost.Count
		}
	}
	return 0
}
