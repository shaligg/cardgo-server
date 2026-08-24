package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/session"
	goredis "github.com/redis/go-redis/v9"
)

const claimPlayerOwnerScript = `
local previous = redis.call('HGET', KEYS[1], 'server_id')
redis.call('HSET', KEYS[1],
  'server_id', ARGV[1],
  'conn_id', ARGV[2],
  'updated_at', ARGV[3])
redis.call('PEXPIRE', KEYS[1], ARGV[4])
return previous or ''
`

const markPlayerOwnerOfflineScript = `
if redis.call('HGET', KEYS[1], 'server_id') == ARGV[1]
  and redis.call('HGET', KEYS[1], 'conn_id') == ARGV[2] then
  redis.call('PEXPIRE', KEYS[1], ARGV[3])
  return 1
end
return 0
`

const refreshPlayerOwnerScript = `
if redis.call('HGET', KEYS[1], 'server_id') == ARGV[1] then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
  return 1
end
return 0
`

// PlayerOwnerStore 使用 Redis 保存玩家当前所在的 GameServer。
type PlayerOwnerStore struct {
	client *Client
	prefix string
}

// NewPlayerOwnerStore 创建 Redis 玩家归属存储。
func NewPlayerOwnerStore(client *Client, prefix string) *PlayerOwnerStore {
	return &PlayerOwnerStore{client: client, prefix: prefix}
}

// Claim 在 GameServer 验票并建立本地会话后原子覆盖玩家归属。
func (s *PlayerOwnerStore) Claim(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) (string, error) {
	if err := s.validate(uid, serverID, ttl); err != nil || connID == "" {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("player owner conn_id is empty")
	}
	previous, err := s.client.raw.Eval(ctx, claimPlayerOwnerScript, []string{s.key(uid)},
		serverID, connID, time.Now().Unix(), ttl.Milliseconds()).Text()
	if err != nil && err != goredis.Nil {
		return "", fmt.Errorf("claim player owner uid=%s: %w", uid, err)
	}
	return previous, nil
}

// MarkOffline 仅在连接仍是当前归属连接时缩短 Redis 记录的离线 TTL。
func (s *PlayerOwnerStore) MarkOffline(ctx context.Context, uid string, serverID string, connID string, ttl time.Duration) error {
	if err := s.validate(uid, serverID, ttl); err != nil {
		return err
	}
	if connID == "" {
		return fmt.Errorf("player owner conn_id is empty")
	}
	if err := s.client.raw.Eval(ctx, markPlayerOwnerOfflineScript, []string{s.key(uid)},
		serverID, connID, ttl.Milliseconds()).Err(); err != nil {
		return fmt.Errorf("mark player owner offline uid=%s: %w", uid, err)
	}
	return nil
}

// GetLastServerID 返回登录分配器用于原节点优先的最近有效归属。
func (s *PlayerOwnerStore) GetLastServerID(ctx context.Context, uid string) (string, bool, error) {
	if uid == "" {
		return "", false, nil
	}
	if err := s.ready(); err != nil {
		return "", false, err
	}
	serverID, err := s.client.raw.HGet(ctx, s.key(uid), "server_id").Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get player owner uid=%s: %w", uid, err)
	}
	return serverID, serverID != "", nil
}

// GetOwners 批量读取玩家归属，缺失或已过期的 UID 不出现在结果中。
func (s *PlayerOwnerStore) GetOwners(ctx context.Context, uids []string) (map[string]session.PlayerOwner, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	owners := make(map[string]session.PlayerOwner, len(uids))
	if len(uids) == 0 {
		return owners, nil
	}
	pipe := s.client.raw.Pipeline()
	commands := make(map[string]*goredis.MapStringStringCmd, len(uids))
	for _, uid := range uids {
		if uid != "" {
			commands[uid] = pipe.HGetAll(ctx, s.key(uid))
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("get player owners: %w", err)
	}
	for uid, command := range commands {
		values, err := command.Result()
		if err != nil || values["server_id"] == "" {
			continue
		}
		owners[uid] = session.PlayerOwner{UID: uid, ServerID: values["server_id"], ConnID: values["conn_id"]}
	}
	return owners, nil
}

// RefreshOwned 批量刷新仍归属于当前节点的在线玩家 TTL，不会覆盖其他节点的新归属。
func (s *PlayerOwnerStore) RefreshOwned(ctx context.Context, serverID string, uids []string, ttl time.Duration) error {
	if err := s.ready(); err != nil {
		return err
	}
	if serverID == "" || ttl <= 0 {
		return fmt.Errorf("invalid player owner refresh server=%s ttl=%s", serverID, ttl)
	}
	if len(uids) == 0 {
		return nil
	}
	pipe := s.client.raw.Pipeline()
	for _, uid := range uids {
		if uid != "" {
			pipe.Eval(ctx, refreshPlayerOwnerScript, []string{s.key(uid)}, serverID, ttl.Milliseconds())
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("refresh player owners server=%s: %w", serverID, err)
	}
	return nil
}

func (s *PlayerOwnerStore) validate(uid string, serverID string, ttl time.Duration) error {
	if err := s.ready(); err != nil {
		return err
	}
	if uid == "" || serverID == "" || ttl <= 0 {
		return fmt.Errorf("invalid player owner uid=%s server=%s ttl=%s", uid, serverID, ttl)
	}
	return nil
}

func (s *PlayerOwnerStore) ready() error {
	if s == nil || s.client == nil || s.client.raw == nil || s.prefix == "" {
		return fmt.Errorf("redis player owner store is not initialized")
	}
	return nil
}

func (s *PlayerOwnerStore) key(uid string) string {
	return s.prefix + ":" + uid
}
