package model

import "time"

// PlayerLevelProgress 保存玩家单个关卡的通关进度。
//
// 同一玩家和关卡只有一行，ClearCount 用于区分首通奖励和重复通关奖励。
type PlayerLevelProgress struct {
	ID             uint   `gorm:"primaryKey"`
	UID            string `gorm:"size:64;not null;uniqueIndex:uk_uid_level"`
	LevelID        int64  `gorm:"not null;uniqueIndex:uk_uid_level;index"`
	ClearCount     int64  `gorm:"not null"`
	FirstClearedAt time.Time
	LastClearedAt  time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
