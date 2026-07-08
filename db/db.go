package db

import (
	"log"

	"github.com/bigfish/go_orm_1/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 全局数据库连接
var DB *gorm.DB

// Init 初始化数据库连接
func Init() error {
	// 获取数据库配置
	dbConfig := config.GlobalConfig.DB

	// 配置GORM日志级别
	logLevel := logger.Warn
	if dbConfig.DebugMode {
		logLevel = logger.Info
	}

	// 初始化数据库连接
	db, err := gorm.Open(sqlite.Open(dbConfig.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return err
	}

	// 配置数据库连接池
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)

	DB = db
	log.Println("Database initialized successfully")
	return nil
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}
