package repo

import (
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

// Migrate 自动迁移当前 MVP 需要的数据库表。
//
// 正式环境可以替换为独立迁移工具，Demo 阶段由启动流程调用此函数。
func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.AutoMigrate(
		&model.Player{}, &model.InventoryItem{}, &model.PlayerCard{}, &model.PlayerDeck{},
		&model.PlayerLevelProgress{}, &model.PlayerWorkshop{}, &model.PlayerFacility{}, &model.AssetLog{},
		&model.FriendRelation{}, &model.Guild{}, &model.GuildMember{}, &model.GuildApplication{}, &model.ChatMessage{},
	)
}
