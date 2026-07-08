package player

import (
	"context"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	"github.com/bigfish/go_orm_1/internal/repo"
)

type Service struct {
	Repo   repo.PlayerRepository
	Assets asset.Service
}

func (s Service) QueryProfile(ctx context.Context, uid string) (repo.Player, error) {
	return s.Repo.GetByUID(ctx, uid)
}

func (s Service) AddGold(ctx context.Context, uid string, delta int64, reqID string) (repo.Player, error) {
	res, err := s.Assets.Grant(ctx, uid, []asset.RewardItem{{ItemID: gamedata.ItemIDGold, Count: delta}}, "player.add_gold", reqID)
	if err != nil {
		return repo.Player{}, err
	}
	return *res[0].Player, nil
}

func (s Service) ConsumeGold(ctx context.Context, uid string, amount int64, reqID string) (repo.Player, error) {
	res, err := s.Assets.Consume(ctx, uid, []asset.CostItem{{ItemID: gamedata.ItemIDGold, Count: amount}}, "player.consume_gold", reqID)
	if err != nil {
		return repo.Player{}, err
	}
	return *res[0].Player, nil
}
