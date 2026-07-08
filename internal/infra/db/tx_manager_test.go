package db

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type txManagerTestRow struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func newTxManagerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(&txManagerTestRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gdb
}

func TestTxManagerDoCommitsOnNilError(t *testing.T) {
	gdb := newTxManagerTestDB(t)
	manager := NewTxManager(gdb)

	err := manager.Do(context.Background(), func(tx *gorm.DB) error {
		return tx.Create(&txManagerTestRow{Name: "commit"}).Error
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}

	var count int64
	if err := gdb.Model(&txManagerTestRow{}).Where("name = ?", "commit").Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
}

func TestTxManagerDoRollsBackOnError(t *testing.T) {
	gdb := newTxManagerTestDB(t)
	manager := NewTxManager(gdb)
	wantErr := errors.New("rollback")

	err := manager.Do(context.Background(), func(tx *gorm.DB) error {
		if err := tx.Create(&txManagerTestRow{Name: "rollback"}).Error; err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var count int64
	if err := gdb.Model(&txManagerTestRow{}).Where("name = ?", "rollback").Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0", count)
	}
}
