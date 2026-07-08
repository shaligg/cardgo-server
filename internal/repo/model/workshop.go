package model

import "time"

// PlayerWorkshop 是玩家工坊基础数据表。
//
// 当前只保存工坊总览所需的全局字段；设施等级、装饰实例等放在各自独立表中。
type PlayerWorkshop struct {
	UID                 string `gorm:"primaryKey;size:64"`
	Level               int    `gorm:"not null"`
	ActiveThemeID       string `gorm:"size:64"`
	LastOfflineRewardAt time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PlayerFacility 是玩家工坊设施数据表。
//
// 每个玩家同一个 facility_id 只有一条记录，设施升级只更新该行等级和解锁状态。
type PlayerFacility struct {
	ID         uint   `gorm:"primaryKey"`
	UID        string `gorm:"size:64;not null;uniqueIndex:uk_uid_facility"`
	FacilityID string `gorm:"size:64;not null;uniqueIndex:uk_uid_facility;index"`
	Level      int    `gorm:"not null"`
	Unlocked   bool   `gorm:"not null"`
	UnlockedAt *time.Time
	UpdatedAt  time.Time
}
