package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/login"
)

// NodeRegistry 使用 Redis 保存所有 GameServer 的临时运行状态。
type NodeRegistry struct {
	client *Client
	prefix string
}

// NewNodeRegistry 创建 Redis 节点注册表。
func NewNodeRegistry(client *Client, prefix string) *NodeRegistry {
	return &NodeRegistry{client: client, prefix: prefix}
}

// UpsertNode 注册或刷新节点状态及 TTL。
func (r *NodeRegistry) UpsertNode(ctx context.Context, node login.NodeInfo, ttl time.Duration) error {
	if r == nil || r.client == nil || r.client.raw == nil {
		return fmt.Errorf("redis node registry is not initialized")
	}
	if node.ServerID == "" || node.WSAddr == "" || ttl <= 0 {
		return fmt.Errorf("invalid game server node")
	}
	pipe := r.client.raw.TxPipeline()
	pipe.SAdd(ctx, r.indexKey(), node.ServerID)
	pipe.HSet(ctx, r.nodeKey(node.ServerID), map[string]interface{}{
		"server_id":  node.ServerID,
		"ws_addr":    node.WSAddr,
		"online":     node.Online,
		"max_online": node.MaxOnline,
		"healthy":    boolInt(node.Healthy),
		"drain":      boolInt(node.Drain),
		"region":     node.Region,
	})
	pipe.Expire(ctx, r.nodeKey(node.ServerID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("upsert game server node %s: %w", node.ServerID, err)
	}
	return nil
}

// RemoveNode 从节点注册表删除指定 GameServer。
func (r *NodeRegistry) RemoveNode(ctx context.Context, serverID string) error {
	if r == nil || r.client == nil || r.client.raw == nil || serverID == "" {
		return nil
	}
	pipe := r.client.raw.TxPipeline()
	pipe.Del(ctx, r.nodeKey(serverID))
	pipe.SRem(ctx, r.indexKey(), serverID)
	_, err := pipe.Exec(ctx)
	return err
}

// ListNodes 返回当前 TTL 尚未过期的全部 GameServer。
func (r *NodeRegistry) ListNodes(ctx context.Context) ([]login.NodeInfo, error) {
	if r == nil || r.client == nil || r.client.raw == nil {
		return nil, fmt.Errorf("redis node registry is not initialized")
	}
	serverIDs, err := r.client.raw.SMembers(ctx, r.indexKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("list game server node ids: %w", err)
	}
	if len(serverIDs) == 0 {
		return nil, nil
	}

	pipe := r.client.raw.Pipeline()
	commands := make(map[string]interface {
		Result() (map[string]string, error)
	}, len(serverIDs))
	for _, serverID := range serverIDs {
		commands[serverID] = pipe.HGetAll(ctx, r.nodeKey(serverID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("load game server nodes: %w", err)
	}

	nodes := make([]login.NodeInfo, 0, len(serverIDs))
	staleIDs := make([]interface{}, 0)
	for _, serverID := range serverIDs {
		values, err := commands[serverID].Result()
		if err != nil || len(values) == 0 {
			staleIDs = append(staleIDs, serverID)
			continue
		}
		node, err := decodeNodeInfo(values)
		if err != nil {
			staleIDs = append(staleIDs, serverID)
			continue
		}
		nodes = append(nodes, node)
	}
	if len(staleIDs) > 0 {
		_ = r.client.raw.SRem(ctx, r.indexKey(), staleIDs...).Err()
	}
	return nodes, nil
}

func decodeNodeInfo(values map[string]string) (login.NodeInfo, error) {
	online, err := strconv.Atoi(values["online"])
	if err != nil {
		return login.NodeInfo{}, err
	}
	maxOnline, err := strconv.Atoi(values["max_online"])
	if err != nil {
		return login.NodeInfo{}, err
	}
	node := login.NodeInfo{
		ServerID:  values["server_id"],
		WSAddr:    values["ws_addr"],
		Online:    online,
		MaxOnline: maxOnline,
		Healthy:   values["healthy"] == "1",
		Drain:     values["drain"] == "1",
		Region:    values["region"],
	}
	if node.ServerID == "" || node.WSAddr == "" {
		return login.NodeInfo{}, fmt.Errorf("node identity is empty")
	}
	return node, nil
}

func (r *NodeRegistry) indexKey() string {
	return r.prefix + ":index"
}

func (r *NodeRegistry) nodeKey(serverID string) string {
	return r.prefix + ":" + serverID
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
