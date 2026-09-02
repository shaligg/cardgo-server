package db

import (
	"os"
	"testing"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
)

func TestOpenCollectsDBMetrics(t *testing.T) {
	dsn := os.Getenv("GAME_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 GAME_TEST_DB_DSN，跳过 MySQL 集成测试")
	}
	registry := metrics.NewRegistry()
	gdb, err := Open(Config{DSN: dsn, Metrics: registry})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := gdb.Exec("SELECT 1").Error; err != nil {
		t.Fatalf("execute query: %v", err)
	}

	snapshot := registry.Snapshot()
	if snapshot.DBRequests == 0 {
		t.Fatal("db_requests was not recorded")
	}
}

func TestOpenRejectsEmptyDSN(t *testing.T) {
	if _, err := Open(Config{}); err == nil {
		t.Fatal("Open(Config{}) returned nil error")
	}
}
