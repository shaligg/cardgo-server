package session

import (
	"context"
	"time"
)

// PlayerOwner 表示 Redis 中记录的玩家当前 GameServer 归属。
type PlayerOwner struct {
	UID      string
	ServerID string
	ConnID   string
}

// PlayerOwnerStore 提供玩家归属认领、查询和 TTL 维护能力。
type PlayerOwnerStore interface {
	Claim(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) (previousServerID string, err error)
	MarkOffline(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) error
	GetLastServerID(ctx context.Context, uid string) (serverID string, ok bool, err error)
	GetOwners(ctx context.Context, uids []string) (map[string]PlayerOwner, error)
	RefreshOwned(ctx context.Context, serverID string, uids []string, ttl time.Duration) error
}
