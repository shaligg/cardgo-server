// Package repo 定义业务层依赖的持久化接口和领域数据结构。
//
// 具体实现负责事务、资产流水和缓存失效边界。
package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Player 是业务层使用的玩家基础数据快照。
//
// 它不是数据库模型，避免上层业务直接依赖 GORM 字段或表结构。
type Player struct {
	UID   string
	Level int
	Gold  int64
}

// InventoryItem 是业务层使用的通用可堆叠背包项。
type InventoryItem struct {
	UID    string
	ItemID int64
	Count  int64
}

// ErrInvalidReqID 表示资产变更缺少用于审计的请求 ID。
var ErrInvalidReqID = errors.New("invalid req_id")

// ErrInvalidAmount 表示资产变更数量非法。
var ErrInvalidAmount = errors.New("invalid amount")

// ErrInsufficientGold 表示玩家金币不足。
var ErrInsufficientGold = errors.New("insufficient gold")

// ErrInsufficientItem 表示玩家背包道具数量不足。
var ErrInsufficientItem = errors.New("insufficient item")

// ErrCardNotOwned 表示玩家尚未拥有目标卡牌。
var ErrCardNotOwned = errors.New("card not owned")

// ErrDeckNotFound 表示目标卡组不存在。
var ErrDeckNotFound = errors.New("deck not found")

// ErrCardMaxLevel 表示卡牌已经达到当前版本等级上限。
var ErrCardMaxLevel = errors.New("card already max level")

// ErrFacilityMaxLevel 表示设施已经达到当前版本等级上限。
var ErrFacilityMaxLevel = errors.New("facility already max level")

// PlayerRepository 定义玩家基础数据的持久化能力。
//
// ChangeGold 必须在实现中保证资产变更和资产流水处于同一事务。
type PlayerRepository interface {
	GetByUID(ctx context.Context, uid string) (Player, error)
	ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error)
}

// TxPlayerRepository 定义可加入外部事务的玩家资产写入能力。
type TxPlayerRepository interface {
	ChangeGoldInTx(ctx context.Context, tx *gorm.DB, uid string, delta int64, itemID int64, reason string, reqID string) (Player, error)
}

// InventoryRepository 定义通用可堆叠背包的持久化能力。
//
// ChangeInventoryItem 必须在实现中保证扣费不为负，并让资产变更和流水处于同一事务。
type InventoryRepository interface {
	GetInventory(ctx context.Context, uid string) ([]InventoryItem, error)
	ChangeInventoryItem(ctx context.Context, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error)
}

// TxInventoryRepository 定义可加入外部事务的背包写入能力。
type TxInventoryRepository interface {
	ChangeInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64, delta int64, reason string, reqID string) (InventoryItem, error)
}

// PlayerCacheInvalidator 定义玩家基础缓存失效能力。
type PlayerCacheInvalidator interface {
	InvalidatePlayer(uid string)
}

// PlayerCard 是业务层使用的玩家卡牌拥有记录。
type PlayerCard struct {
	UID    string `json:"uid"`
	CardID int64  `json:"card_id"`
	Level  int    `json:"level"`
	Exp    int64  `json:"exp"`
	Count  int64  `json:"count"`
}

// PlayerDeck 是业务层使用的玩家卡组方案。
type PlayerDeck struct {
	UID      string  `json:"uid"`
	DeckID   int32   `json:"deck_id"`
	Name     string  `json:"name,omitempty"`
	CardIDs  []int64 `json:"card_ids"`
	IsActive bool    `json:"is_active"`
}

// CardRepository 定义卡牌库存与卡组的持久化能力。
type CardRepository interface {
	GetCards(ctx context.Context, uid string) ([]PlayerCard, error)
	GetDeck(ctx context.Context, uid string, deckID int32) (PlayerDeck, error)
	EnsureDefaultCards(ctx context.Context, uid string, cardIDs []int64) error
	SaveDeck(ctx context.Context, uid string, deckID int32, name string, cardIDs []int64) (PlayerDeck, error)
	UpgradeCard(ctx context.Context, uid string, cardID int64, maxLevel int) (PlayerCard, error)
	UpgradeCardInTx(ctx context.Context, tx *gorm.DB, uid string, cardID int64, maxLevel int) (PlayerCard, error)
}

// PlayerWorkshop 是业务层使用的玩家工坊基础数据。
type PlayerWorkshop struct {
	UID                 string `json:"uid"`
	Level               int    `json:"level"`
	ActiveThemeID       string `json:"active_theme_id"`
	LastOfflineRewardAt int64  `json:"last_offline_reward_at"`
}

// OfflineRewardClaim 是业务层使用的离线收益领取结果。
type OfflineRewardClaim struct {
	UID              string `json:"uid"`
	OfflineSeconds   int64  `json:"offline_seconds"`
	EffectiveSeconds int64  `json:"effective_seconds"`
	Gold             int64  `json:"gold"`
	BasicMaterial    int64  `json:"basic_material"`
	ClaimedAt        int64  `json:"claimed_at"`
}

// PlayerFacility 是业务层使用的玩家工坊设施数据。
type PlayerFacility struct {
	UID        string `json:"uid"`
	FacilityID string `json:"facility_id"`
	Level      int    `json:"level"`
	Unlocked   bool   `json:"unlocked"`
	UnlockedAt int64  `json:"unlocked_at,omitempty"`
}

// WorkshopRepository 定义工坊总览需要的持久化能力。
type WorkshopRepository interface {
	GetOrCreateWorkshop(ctx context.Context, uid string) (PlayerWorkshop, error)
	GetFacilities(ctx context.Context, uid string) ([]PlayerFacility, error)
	GetOrCreateFacility(ctx context.Context, uid string, facilityID string) (PlayerFacility, error)
	UpgradeFacility(ctx context.Context, uid string, facilityID string, maxLevel int) (PlayerFacility, error)
	UpgradeFacilityInTx(ctx context.Context, tx *gorm.DB, uid string, facilityID string, maxLevel int) (PlayerFacility, error)
	RecordOfflineRewardClaim(ctx context.Context, uid string, claim OfflineRewardClaim) (OfflineRewardClaim, error)
	RecordOfflineRewardClaimInTx(ctx context.Context, tx *gorm.DB, uid string, claim OfflineRewardClaim) (OfflineRewardClaim, error)
}
