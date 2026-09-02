// Package testdb 提供只用于测试的 MySQL 数据库入口。
package testdb

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var prefixSequence atomic.Uint64

// Open 连接 GAME_TEST_DB_DSN，并用独立表前缀隔离当前测试的数据表。
func Open(t testing.TB, models ...interface{}) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("GAME_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 GAME_TEST_DB_DSN，跳过 MySQL 集成测试")
	}

	prefix := fmt.Sprintf("t_%x_%x_", time.Now().UnixNano(), prefixSequence.Add(1))
	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: prefix},
	})
	if err != nil {
		t.Fatalf("连接 MySQL 测试库失败: %v", err)
	}
	if err := gdb.AutoMigrate(models...); err != nil {
		t.Fatalf("迁移 MySQL 测试表失败: %v", err)
	}

	t.Cleanup(func() {
		if err := gdb.Migrator().DropTable(models...); err != nil {
			t.Errorf("清理 MySQL 测试表失败: %v", err)
		}
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return gdb
}

// OpenGame 创建包含当前游戏服全部持久化模型的测试数据库。
func OpenGame(t testing.TB) *gorm.DB {
	t.Helper()
	return Open(t,
		&model.Player{},
		&model.InventoryItem{},
		&model.PlayerCard{},
		&model.PlayerDeck{},
		&model.PlayerWorkshop{},
		&model.PlayerFacility{},
		&model.AssetLog{},
	)
}
