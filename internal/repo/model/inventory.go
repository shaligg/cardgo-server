package model

import "time"

// InventoryItem 是通用可堆叠背包表。
//
// 一名玩家同一个 item_id 只有一行记录，不能堆叠的道具后续应按具体系统单独建表。
type InventoryItem struct {
	ID        uint   `gorm:"primaryKey"`
	UID       string `gorm:"size:64;not null;uniqueIndex:uk_uid_item"`
	ItemID    int64  `gorm:"not null;uniqueIndex:uk_uid_item;index"`
	Count     int64  `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
