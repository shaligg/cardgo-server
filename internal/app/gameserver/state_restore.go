package gameserver

import (
	"context"

	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

// buildRestoreStateCallback 优先复用本节点内存；内存不存在时从正式玩家表重建。
func buildRestoreStateCallback(online *state.OnlineState, players repo.PlayerRepository) func(ctx context.Context, uid string) (map[string]interface{}, bool) {
	return func(ctx context.Context, uid string) (map[string]interface{}, bool) {
		if online != nil {
			if st, ok := online.Get(uid); ok {
				online.MarkOnline(uid)
				return cloneMap(st.Data), true
			}
		}
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

		if online != nil {
			online.Set(state.PlayerState{
				UID:     uid,
				Version: 1,
				Data:    cloneMap(data),
			})
		}
		return data, true
	}
}

func cloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
