package gameserver

import (
	"context"

	playergame "github.com/bigfish/go_orm_1/internal/domain/player"
)

// buildPreparePlayerCallback 在会话绑定前初始化玩家，并构造鉴权后的基础同步数据。
func buildPreparePlayerCallback(players playergame.Service) func(ctx context.Context, uid string) (map[string]interface{}, error) {
	return func(ctx context.Context, uid string) (map[string]interface{}, error) {
		player, err := players.EnsureCreated(ctx, uid)
		if err != nil {
			return nil, err
		}
		data := map[string]interface{}{
			"uid":       player.UID,
			"nickname":  player.Nickname,
			"avatar_id": player.AvatarID,
			"level":     player.Level,
			"gold":      player.Gold,
		}
		return data, nil
	}
}
