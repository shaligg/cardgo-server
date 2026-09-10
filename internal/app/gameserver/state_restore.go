package gameserver

import (
	"context"

	"github.com/bigfish/go_orm_1/internal/repo"
)

// buildRestoreStateCallback 从正式玩家表构造鉴权后的基础同步数据。
func buildRestoreStateCallback(players repo.PlayerRepository) func(ctx context.Context, uid string) (map[string]interface{}, bool) {
	return func(ctx context.Context, uid string) (map[string]interface{}, bool) {
		if players == nil {
			return nil, false
		}
		player, err := players.GetByUID(ctx, uid)
		if err != nil {
			return nil, false
		}
		data := map[string]interface{}{
			"uid":   player.UID,
			"level": player.Level,
			"gold":  player.Gold,
		}
		return data, true
	}
}
