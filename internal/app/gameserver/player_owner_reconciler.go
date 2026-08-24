package gameserver

import (
	"context"
	"time"

	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	battlegame "github.com/bigfish/go_orm_1/internal/game/battle"
	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// playerOwnerReconciler 定期清理已经迁移到其他 GameServer 的本机玩家运行时。
type playerOwnerReconciler struct {
	nodeID   string
	ownerTTL time.Duration
	owners   session.PlayerOwnerStore
	sessions *session.MemoryManager
	online   *state.OnlineState
	battles  *battlegame.Service
	wsServer *ws.Server
}

func (r *playerOwnerReconciler) ReconcileOwners(ctx context.Context) {
	uids, activeUIDs := r.localUIDs(ctx)
	if len(uids) == 0 || r.owners == nil {
		return
	}
	owners, err := r.owners.GetOwners(ctx, uids)
	if err != nil {
		ilog.Errorf("reconcile player owners failed node=%s err=%v", r.nodeID, err)
		return
	}

	refreshUIDs := make([]string, 0, len(activeUIDs))
	for _, uid := range uids {
		owner, ok := owners[uid]
		if !ok {
			// Redis 查询成功但离线玩家归属已过期，说明原节点恢复窗口也已结束。
			if !activeUIDs[uid] {
				r.removePlayerRuntime(ctx, uid)
			}
			continue
		}
		if owner.ServerID != r.nodeID {
			r.removePlayerRuntime(ctx, uid)
			continue
		}
		if activeUIDs[uid] {
			refreshUIDs = append(refreshUIDs, uid)
		}
	}
	if err := r.owners.RefreshOwned(ctx, r.nodeID, refreshUIDs, r.ownerTTL); err != nil {
		ilog.Errorf("refresh player owners failed node=%s err=%v", r.nodeID, err)
	}
}

func (r *playerOwnerReconciler) localUIDs(ctx context.Context) ([]string, map[string]bool) {
	seen := map[string]bool{}
	active := map[string]bool{}
	if r.sessions != nil {
		for _, current := range r.sessions.List(ctx) {
			seen[current.UID] = true
			active[current.UID] = true
		}
	}
	if r.online != nil {
		for _, current := range r.online.List() {
			seen[current.UID] = true
		}
	}
	if r.battles != nil {
		for _, uid := range r.battles.PlayerUIDs() {
			seen[uid] = true
		}
	}
	uids := make([]string, 0, len(seen))
	for uid := range seen {
		if uid != "" {
			uids = append(uids, uid)
		}
	}
	return uids, active
}

func (r *playerOwnerReconciler) removePlayerRuntime(ctx context.Context, uid string) {
	if r.wsServer != nil {
		r.wsServer.KickUID(ctx, uid, "player connected to another game server")
	}
	if r.online != nil {
		r.online.Delete(uid)
	}
	if r.battles != nil {
		r.battles.DeletePlayerRuntime(uid)
	}
}
