package model

import "time"

// Account 保存内部 UID、账号状态及当前登录凭证，不承担登录历史。
type Account struct {
	UID       string `gorm:"primaryKey;size:64"`
	Status    string `gorm:"size:16;not null"`
	TokenHash string `gorm:"type:char(64);not null"`
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AccountIdentity 以来源和规范化身份唯一映射内部 UID。
type AccountIdentity struct {
	Provider     string `gorm:"primaryKey;size:32"`
	Subject      string `gorm:"primaryKey;size:64"`
	UID          string `gorm:"size:64;not null;index"`
	PasswordHash string `gorm:"size:128;not null"`
	CreatedAt    time.Time
}
