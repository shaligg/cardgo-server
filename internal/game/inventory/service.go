// Package inventory 提供玩家通用可堆叠背包的查询能力。
//
// 背包写入统一走 asset 模块，以保证幂等、余额校验和资产流水一致。
package inventory

import (
	"context"

	"github.com/bigfish/go_orm_1/internal/repo"
)

// Service 是背包查询服务。
//
// 当前只负责读背包；增减道具由 asset.Service 统一处理。
type Service struct {
	Repo repo.InventoryRepository
}

// GetInventory 查询玩家当前的通用可堆叠背包。
func (s Service) GetInventory(ctx context.Context, uid string) ([]repo.InventoryItem, error) {
	return s.Repo.GetInventory(ctx, uid)
}
