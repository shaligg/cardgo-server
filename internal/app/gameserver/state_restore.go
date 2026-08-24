package gameserver

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/platform/state"
)

func buildRestoreStateCallback(online *state.OnlineState, snapshots state.SnapshotStore) func(ctx context.Context, uid string) (map[string]interface{}, bool) {
	return func(ctx context.Context, uid string) (map[string]interface{}, bool) {
		if online != nil {
			if st, ok := online.Get(uid); ok {
				online.MarkOnline(uid)
				return cloneMap(st.Data), true
			}
		}
		if snapshots == nil {
			return nil, false
		}

		snap, ok, err := snapshots.LoadSnapshot(ctx, uid)
		if err != nil || !ok {
			return nil, false
		}

		var m map[string]interface{}
		if len(snap.Payload) > 0 {
			if err := json.Unmarshal(snap.Payload, &m); err != nil {
				return nil, false
			}
		}
		if m == nil {
			m = map[string]interface{}{}
		}

		if online != nil {
			online.Set(state.PlayerState{
				UID:     uid,
				Version: snap.Version,
				Data:    cloneMap(m),
			})
		}
		return m, true
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
