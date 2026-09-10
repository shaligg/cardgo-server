// Package player 实现玩家建号和基础资料领域逻辑。
package player

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/bigfish/go_orm_1/internal/domain/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	"github.com/bigfish/go_orm_1/internal/repo"
)

const (
	defaultPlayerLevel    = 1
	defaultPlayerAvatarID = 1
)

// Service 提供玩家基础资料能力，并通过资产领域处理货币变更。
type Service struct {
	Repo   repo.PlayerRepository
	Assets asset.Service
}

// EnsureCreated 显式、幂等地初始化新玩家基础资料。
func (s Service) EnsureCreated(ctx context.Context, uid string) (repo.Player, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return repo.Player{}, repo.ErrPlayerNotFound
	}
	player, err := s.Repo.GetByUID(ctx, uid)
	if err == nil {
		return player, nil
	}
	if !errors.Is(err, repo.ErrPlayerNotFound) {
		return repo.Player{}, err
	}
	return s.Repo.CreateIfAbsent(ctx, repo.Player{
		UID:      uid,
		Nickname: defaultNickname(uid),
		AvatarID: defaultPlayerAvatarID,
		Level:    defaultPlayerLevel,
		Gold:     0,
	})
}

// QueryProfile 查询已存在玩家的基础资料。
func (s Service) QueryProfile(ctx context.Context, uid string) (repo.Player, error) {
	return s.Repo.GetByUID(ctx, uid)
}

// AddGold 通过统一资产领域增加玩家金币。
func (s Service) AddGold(ctx context.Context, uid string, delta int64, reqID string) (repo.Player, error) {
	res, err := s.Assets.Grant(ctx, uid, []asset.RewardItem{{ItemID: gamedata.ItemIDGold, Count: delta}}, "player.add_gold", reqID)
	if err != nil {
		return repo.Player{}, err
	}
	return *res[0].Player, nil
}

// ConsumeGold 通过统一资产领域扣除玩家金币。
func (s Service) ConsumeGold(ctx context.Context, uid string, amount int64, reqID string) (repo.Player, error) {
	res, err := s.Assets.Consume(ctx, uid, []asset.CostItem{{ItemID: gamedata.ItemIDGold, Count: amount}}, "player.consume_gold", reqID)
	if err != nil {
		return repo.Player{}, err
	}
	return *res[0].Player, nil
}

func defaultNickname(uid string) string {
	sum := sha256.Sum256([]byte(uid))
	return fmt.Sprintf("Guest_%x", sum[:4])
}
