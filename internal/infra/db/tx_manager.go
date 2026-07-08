package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// TxManager 统一管理一次数据库事务的开启、提交和回滚。
//
// 它只负责事务生命周期，不包含任何玩法逻辑；业务层在 fn 内用同一个 tx 编排多个 repo/writer。
type TxManager struct {
	db *gorm.DB
}

// NewTxManager 创建事务管理器。
func NewTxManager(db *gorm.DB) TxManager {
	return TxManager{db: db}
}

// Do 在同一个事务内执行 fn。
//
// fn 返回 nil 时提交事务；返回 error 时 GORM 自动回滚事务。
func (m TxManager) Do(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if m.db == nil {
		return fmt.Errorf("db is nil")
	}
	if fn == nil {
		return fmt.Errorf("transaction function is nil")
	}
	return m.db.WithContext(ctx).Transaction(fn)
}
