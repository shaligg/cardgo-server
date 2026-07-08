package model

import "time"

// IdempotencyRecord 是幂等请求记录表。
//
// 资产变更按 uid/action/req_id 去重，重复请求会返回第一次执行保存的结果。
type IdempotencyRecord struct {
	ID         uint   `gorm:"primaryKey"`
	UID        string `gorm:"size:64;not null;uniqueIndex:uk_uid_action_reqid"`
	Action     string `gorm:"size:64;not null;uniqueIndex:uk_uid_action_reqid"`
	ReqID      string `gorm:"size:128;not null;uniqueIndex:uk_uid_action_reqid"`
	ResultJSON string `gorm:"type:text"`
	CreatedAt  time.Time
}
