package login

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"time"
)

// ErrNoAvailableNode 表示当前没有可分配的 GameServer。
//
// 登录层收到这个错误后应返回“暂无可用服务器”或让客户端稍后重试。
var ErrNoAvailableNode = errors.New("no available game server")

// NodeInfo 是登录服看到的 GameServer 节点状态。
//
// 当前由 GameServer 定时上报到 Redis，LoginService 每次分配时读取存活节点。
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
// LoginService 只依赖此接口，不感知节点来自 Redis 或其他服务发现组件。
type NodeRegistry interface {
	ListNodes(ctx context.Context) ([]NodeInfo, error)
}

// NodeRegistrar 负责注册、刷新和注销 GameServer 运行状态。
type NodeRegistrar interface {
	UpsertNode(ctx context.Context, node NodeInfo, ttl time.Duration) error
	RemoveNode(ctx context.Context, serverID string) error
}

// LastServerReader 读取玩家最近一次成功进入的 GameServer。
//
// 该接口只读；归属只能由 GameServer 在鉴权并绑定会话成功后更新。
type LastServerReader interface {
	GetLastServerID(ctx context.Context, uid string) (serverID string, ok bool, err error)
}

// StaticNodeRegistry 是基于静态配置的节点注册表。
//
// 它只用于分配器单元测试，不进入当前运行链路。
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
// 规则：重连优先原服；原服不可用时，从健康、非 drain、未满载节点中做负载感知的两选一分配。
type RegistryNodeAllocator struct {
	Registry   NodeRegistry
	LastServer LastServerReader
}

// Allocate 为玩家选择一个 GameServer。
func (a RegistryNodeAllocator) Allocate(ctx context.Context, uid string, clientIP string) (string, string, error) {
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

	allocationKey := uid
	if allocationKey == "" {
		allocationKey = clientIP
	}
	node, ok := pickLoadAwareNode(nodes, allocationKey)
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

// pickLoadAwareNode 使用稳定的“两选一”策略，避免心跳数据相同时所有登录都涌向同一节点。
func pickLoadAwareNode(nodes []NodeInfo, allocationKey string) (NodeInfo, bool) {
	available := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		if node.Available() {
			available = append(available, node)
		}
	}
	if len(available) == 0 {
		return NodeInfo{}, false
	}
	if len(available) == 1 {
		return available[0], true
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].ServerID < available[j].ServerID
	})
	firstIndex := int(hashString(allocationKey) % uint32(len(available)))
	secondOffset := 1 + int(hashString(allocationKey+"#second")%uint32(len(available)-1))
	secondIndex := (firstIndex + secondOffset) % len(available)
	first := available[firstIndex]
	second := available[secondIndex]
	if lessLoad(second, first) {
		return second, true
	}
	return first, true
}

func lessLoad(a NodeInfo, b NodeInfo) bool {
	aRatio := loadRatio(a)
	bRatio := loadRatio(b)
	if aRatio != bRatio {
		return aRatio < bRatio
	}
	return a.Online < b.Online
}

func loadRatio(n NodeInfo) float64 {
	if n.MaxOnline <= 0 {
		return 0
	}
	return float64(n.Online) / float64(n.MaxOnline)
}

func hashString(value string) uint32 {
	sum := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint32(sum[:4])
}
