package login

import (
	"context"
	"errors"
)

// ErrNoAvailableNode 表示当前没有可分配的 GameServer。
//
// 登录层收到这个错误后应返回“暂无可用服务器”或让客户端稍后重试。
var ErrNoAvailableNode = errors.New("no available game server")

// NodeInfo 是登录服看到的 GameServer 节点状态。
//
// MVP 阶段可以由静态配置提供；多节点阶段通常由 GameServer 心跳上报到 Redis/DB 后提供。
type NodeInfo struct {
	ServerID  string
	WSAddr    string
	Online    int
	MaxOnline int
	Healthy   bool
	Drain     bool
	Region    string
}

// Available 判断节点是否可以接收新登录或重连玩家。
func (n NodeInfo) Available() bool {
	if n.ServerID == "" || n.WSAddr == "" {
		return false
	}
	if !n.Healthy || n.Drain {
		return false
	}
	return n.MaxOnline <= 0 || n.Online < n.MaxOnline
}

// NodeRegistry 提供当前 GameServer 节点列表。
//
// 这个接口是未来多 GameServer、服务发现和 AccessGateway 路由的共同基础。
type NodeRegistry interface {
	ListNodes(ctx context.Context) ([]NodeInfo, error)
}

// LastServerStore 记录玩家上一次所在的 GameServer。
//
// 重连时 NodeAllocator 会优先尝试把玩家分配回原服，以便恢复本机内存热状态。
type LastServerStore interface {
	GetLastServerID(ctx context.Context, uid string) (serverID string, ok bool, err error)
}

// StaticNodeRegistry 是基于静态配置的节点注册表。
//
// 当前 demo 用它提供单节点能力；未来可以替换为 Redis/DB/服务发现实现。
type StaticNodeRegistry struct {
	Nodes []NodeInfo
}

// ListNodes 返回静态节点列表的副本，避免调用方修改内部配置。
func (r StaticNodeRegistry) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	_ = ctx
	out := make([]NodeInfo, len(r.Nodes))
	copy(out, r.Nodes)
	return out, nil
}

// NodeAllocator 负责为登录或重连玩家选择目标 GameServer。
type NodeAllocator interface {
	Allocate(ctx context.Context, uid string, clientIP string) (serverID string, wsAddr string, err error)
}

// RegistryNodeAllocator 基于 NodeRegistry 选择 GameServer。
//
// 规则：重连优先原服；原服不可用时，选择健康、非 drain、未满载且负载最低的节点。
type RegistryNodeAllocator struct {
	Registry   NodeRegistry
	LastServer LastServerStore
}

// Allocate 为玩家选择一个 GameServer。
func (a RegistryNodeAllocator) Allocate(ctx context.Context, uid string, clientIP string) (string, string, error) {
	_ = clientIP
	if a.Registry == nil {
		return "", "", ErrNoAvailableNode
	}
	nodes, err := a.Registry.ListNodes(ctx)
	if err != nil {
		return "", "", err
	}
	if len(nodes) == 0 {
		return "", "", ErrNoAvailableNode
	}

	if a.LastServer != nil && uid != "" {
		lastServerID, ok, err := a.LastServer.GetLastServerID(ctx, uid)
		if err != nil {
			return "", "", err
		}
		if ok {
			if node, ok := findAvailableNode(nodes, lastServerID); ok {
				return node.ServerID, node.WSAddr, nil
			}
		}
	}

	node, ok := pickLowestLoadNode(nodes)
	if !ok {
		return "", "", ErrNoAvailableNode
	}
	return node.ServerID, node.WSAddr, nil
}

// SingleNodeAllocator 是最小 demo 分配器。
//
// 新代码优先使用 RegistryNodeAllocator；保留它是为了简单测试或脚本场景。
type SingleNodeAllocator struct {
	ServerID string
	WSAddr   string
}

// Allocate 总是返回配置中的单个 GameServer。
func (a SingleNodeAllocator) Allocate(ctx context.Context, uid string, clientIP string) (string, string, error) {
	_ = ctx
	_ = uid
	_ = clientIP
	return a.ServerID, a.WSAddr, nil
}

func findAvailableNode(nodes []NodeInfo, serverID string) (NodeInfo, bool) {
	for _, node := range nodes {
		if node.ServerID == serverID && node.Available() {
			return node, true
		}
	}
	return NodeInfo{}, false
}

func pickLowestLoadNode(nodes []NodeInfo) (NodeInfo, bool) {
	var best NodeInfo
	found := false
	for _, node := range nodes {
		if !node.Available() {
			continue
		}
		if !found || lessLoad(node, best) {
			best = node
			found = true
		}
	}
	return best, found
}

func lessLoad(a NodeInfo, b NodeInfo) bool {
	aRatio := loadRatio(a)
	bRatio := loadRatio(b)
	if aRatio != bRatio {
		return aRatio < bRatio
	}
	if a.Online != b.Online {
		return a.Online < b.Online
	}
	return a.ServerID < b.ServerID
}

func loadRatio(n NodeInfo) float64 {
	if n.MaxOnline <= 0 {
		return 0
	}
	return float64(n.Online) / float64(n.MaxOnline)
}
