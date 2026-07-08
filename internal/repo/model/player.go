// Package model 定义数据库表对应的 GORM 模型。
//
// 上层业务不直接依赖这些结构，避免表结构调整时影响业务模块。
package model

// Player 是玩家基础数据表。
//
// 当前 MVP 把高频基础货币 gold 放在玩家主表，背包类通用道具放在 inventory_items。
type Player struct {
	UID   string `gorm:"primaryKey"`
	Level int
	Gold  int64
}
