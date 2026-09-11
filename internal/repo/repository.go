// Package repo 定义业务层依赖的持久化接口和领域数据结构。
//
// 具体实现负责事务内持久化、资产流水和数据库模型转换，事务边界由 Service 编排。
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
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
	AvatarID int64  `json:"avatar_id"`
	Level    int    `json:"level"`
	Gold     int64  `json:"gold"`
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

// ErrPlayerNotFound 表示玩家基础资料不存在。
var ErrPlayerNotFound = errors.New("player not found")

// ErrCardNotOwned 表示玩家尚未拥有目标卡牌。
var ErrCardNotOwned = errors.New("card not owned")

// ErrDeckNotFound 表示目标卡组不存在。
var ErrDeckNotFound = errors.New("deck not found")

// ErrPlayerFacilityNotFound 表示玩家还没有目标设施数据。
var ErrPlayerFacilityNotFound = errors.New("player facility not found")

// ErrPlayerWorkshopNotFound 表示玩家还没有工坊基础数据。
var ErrPlayerWorkshopNotFound = errors.New("player workshop not found")

// ErrLevelProgressNotFound 表示玩家还没有目标关卡的进度记录。
var ErrLevelProgressNotFound = errors.New("level progress not found")

// 社交仓储查询和数据库约束转换使用的错误。
var (
	ErrSocialPlayerNotFound     = errors.New("player not found")
	ErrGuildNameTaken           = errors.New("guild name already exists")
	ErrGuildNotFound            = errors.New("guild not found")
	ErrAlreadyInGuild           = errors.New("player already belongs to a guild")
	ErrNotGuildMember           = errors.New("player is not a guild member")
	ErrGuildPermissionDenied    = errors.New("guild permission denied")
	ErrGuildApplicationExists   = errors.New("guild application already exists")
	ErrGuildApplicationNotFound = errors.New("guild application not found")
)

const (
	GuildRoleLeader = "leader"
	GuildRoleMember = "member"
)

// FriendRecord 是好友仓储返回的一条关系记录。
type FriendRecord struct {
	OtherUID     string
	RequesterUID string
	Status       string
}

// FriendRelationRecord 是好友状态流转读取的关系记录。
type FriendRelationRecord struct {
	RequesterUID string
	Status       string
}

// GuildMemberRecord 是公会成员仓储 DTO。
type GuildMemberRecord struct {
	GuildID  string
	UID      string
	Role     string
	JoinedAt int64
}

// GuildRecord 是公会查询仓储 DTO。
type GuildRecord struct {
	GuildID        string
	Name           string
	OwnerUID       string
	MemberCount    int
	MyRole         string
	HasApplication bool
	Members        []GuildMemberRecord
}

// GuildApplicationRecord 是待审批入会申请的仓储 DTO。
type GuildApplicationRecord struct {
	UID       string
	Level     int
	Nickname  string
	AvatarID  int64
	CreatedAt int64
}

// GuildMembershipReader 只暴露聊天解析当前公会频道需要的成员查询。
type GuildMembershipReader interface {
	GetGuildIDByUID(ctx context.Context, uid string) (string, error)
}

// ChatMessageRecord 是聊天仓储 DTO。
type ChatMessageRecord struct {
	MsgID     string
	ChannelID string
	UID       string
	Content   string
	ReqID     string
	CreatedAt int64
}

// ChatRepository 定义聊天消息持久化和历史查询能力。
type ChatRepository interface {
	CreateChatMessage(ctx context.Context, message ChatMessageRecord) (ChatMessageRecord, error)
	ListChatMessages(ctx context.Context, channelID string, beforeID uint64, limit int) ([]ChatMessageRecord, uint64, error)
}

// PlayerRepository 定义玩家基础资料的持久化能力。
type PlayerRepository interface {
	GetByUID(ctx context.Context, uid string) (Player, error)
	CreateIfAbsent(ctx context.Context, player Player) (Player, error)
}

// PlayerAssetRepository 定义玩家主表字段类资产的事务内持久化能力。
type PlayerAssetRepository interface {
	GetPlayerAssetInTx(ctx context.Context, tx *gorm.DB, uid string) (Player, error)
	SaveGoldInTx(ctx context.Context, tx *gorm.DB, uid string, balance int64, itemID int64, delta int64, reason string, reqID string) error
}

// InventoryRepository 定义通用可堆叠背包的查询能力。
type InventoryRepository interface {
	GetInventory(ctx context.Context, uid string) ([]InventoryItem, error)
}

// InventoryAssetRepository 定义通用可堆叠背包资产的事务内持久化能力。
type InventoryAssetRepository interface {
	GetOrCreateInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64) (InventoryItem, error)
	SaveInventoryItemInTx(ctx context.Context, tx *gorm.DB, item InventoryItem, delta int64, reason string, reqID string) error
}

// PlayerLevelProgress 是业务层使用的玩家关卡通关记录。
type PlayerLevelProgress struct {
	UID            string `json:"uid"`
	LevelID        int64  `json:"level_id"`
	ClearCount     int64  `json:"clear_count"`
	FirstClearedAt int64  `json:"first_cleared_at"`
	LastClearedAt  int64  `json:"last_cleared_at"`
}

// LevelProgressRepository 定义关卡结算事务需要的进度持久化能力。
//
// Repository 只读取和保存业务层已经计算完成的数据，不负责递增通关次数等业务规则。
type LevelProgressRepository interface {
	GetLevelProgressInTx(ctx context.Context, tx *gorm.DB, uid string, levelID int64) (PlayerLevelProgress, error)
	CreateLevelProgressInTx(ctx context.Context, tx *gorm.DB, progress PlayerLevelProgress) error
	UpdateLevelProgressInTx(ctx context.Context, tx *gorm.DB, progress PlayerLevelProgress) error
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
	GetCardInTx(ctx context.Context, tx *gorm.DB, uid string, cardID int64) (PlayerCard, error)
	GetDeck(ctx context.Context, uid string, deckID int32) (PlayerDeck, error)
	CreateCardsIfAbsent(ctx context.Context, cards []PlayerCard) error
	SaveDeck(ctx context.Context, deck PlayerDeck) (PlayerDeck, error)
	UpdateCardInTx(ctx context.Context, tx *gorm.DB, card PlayerCard) error
}

// PlayerWorkshop 是业务层使用的玩家工坊基础数据。
type PlayerWorkshop struct {
	UID                 string `json:"uid"`
	Level               int    `json:"level"`
	ActiveThemeID       string `json:"active_theme_id"`
	LastOfflineRewardAt int64  `json:"last_offline_reward_at"`
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
	GetWorkshop(ctx context.Context, uid string) (PlayerWorkshop, error)
	CreateWorkshopIfAbsent(ctx context.Context, workshop PlayerWorkshop) (PlayerWorkshop, error)
	GetFacilities(ctx context.Context, uid string) ([]PlayerFacility, error)
	GetFacilityInTx(ctx context.Context, tx *gorm.DB, uid string, facilityID string) (PlayerFacility, error)
	CreateFacilityInTx(ctx context.Context, tx *gorm.DB, facility PlayerFacility) error
	UpdateFacilityInTx(ctx context.Context, tx *gorm.DB, facility PlayerFacility) error
	UpdateLastOfflineRewardAtInTx(ctx context.Context, tx *gorm.DB, uid string, claimedAt int64) error
}
