package model

import "time"

// PlayerCard 是玩家拥有的卡牌记录。
//
// 当前 MVP 把同一种卡牌聚合成一行；后续如果出现不可堆叠卡牌实例，再单独建实例表。
type PlayerCard struct {
	ID        uint   `gorm:"primaryKey"`
	UID       string `gorm:"size:64;not null;uniqueIndex:uk_uid_card"`
	CardID    int64  `gorm:"not null;uniqueIndex:uk_uid_card;index"`
	Level     int    `gorm:"not null"`
	Exp       int64  `gorm:"not null"`
	Count     int64  `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PlayerDeck 是玩家保存的卡组方案。
//
// CardIDsJSON 用 JSON 保存卡牌 ID 列表，MVP 查询简单；后续需要复杂检索时再拆明细表。
type PlayerDeck struct {
	ID          uint   `gorm:"primaryKey"`
	UID         string `gorm:"size:64;not null;uniqueIndex:uk_uid_deck"`
	DeckID      int32  `gorm:"not null;uniqueIndex:uk_uid_deck"`
	Name        string `gorm:"size:64"`
	CardIDsJSON string `gorm:"type:text;not null"`
	IsActive    bool   `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
