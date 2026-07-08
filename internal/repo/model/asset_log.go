package model

import "time"

// AssetLog 是资产流水表。
//
// 所有通过 asset 模块产生的金币和背包道具变更都应写入这里，方便查账和问题追踪。
type AssetLog struct {
	ID        uint   `gorm:"primaryKey"`
	UID       string `gorm:"size:64;not null;index"`
	ItemID    int64  `gorm:"not null;index"`
	Delta     int64  `gorm:"not null"`
	Balance   int64  `gorm:"not null"`
	Reason    string `gorm:"size:64;not null;index"`
	ReqID     string `gorm:"size:128;not null;index"`
	CreatedAt time.Time
}
