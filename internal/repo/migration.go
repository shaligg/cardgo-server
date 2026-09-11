package repo

import (
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
)

// Migrate 显式准备当前开发和测试需要的数据库表。
//
// 业务进程启动时不得调用；正式环境的完整建表 SQL 在上线前单独准备和执行。
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
