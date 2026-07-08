package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	DSN string
}

func Open(cfg Config) (*gorm.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = "file:game_demo.db?cache=shared&_busy_timeout=5000"
	}

	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return gdb, nil
}
