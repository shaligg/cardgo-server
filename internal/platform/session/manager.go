package session

import (
	"context"
	"sync"
	"time"
)

// Session 表示玩家当前在线连接会话。
//
// MVP 阶段 ConnID 由 GameServer 在 WS 鉴权成功后生成，并直接作为 auth_ack 的 session_id 返回。
type Session struct {
	UID      string
	ConnID   string
	LoginAt  time.Time
	LastSeen time.Time
	ClientIP string
}

// Manager 管理 uid 与当前连接会话的绑定关系。
//
// Bind 返回旧连接 ID，调用方可以据此踢掉同账号的旧连接。
type Manager interface {
	Bind(ctx context.Context, s Session) (oldConnID string, err error)
	GetByUID(ctx context.Context, uid string) (Session, bool, error)
	Unbind(ctx context.Context, uid, connID string) error
	Count(ctx context.Context) (int, error)
}

// MemoryManager 是进程内会话管理器。
//
// 它适合 MVP 单进程使用；多 GameServer 场景下，跨节点查询应通过 Redis/DB 会话索引实现。
type MemoryManager struct {
	mu    sync.RWMutex
	items map[string]Session
}

// NewMemoryManager 创建进程内会话管理器。
func NewMemoryManager() *MemoryManager {
	return &MemoryManager{items: make(map[string]Session)}
}

// Bind 绑定玩家当前连接，并返回该玩家之前的连接 ID。
func (m *MemoryManager) Bind(ctx context.Context, s Session) (string, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.items[s.UID]
	m.items[s.UID] = s
	return old.ConnID, nil
}

// GetByUID 查询玩家当前连接会话。
func (m *MemoryManager) GetByUID(ctx context.Context, uid string) (Session, bool, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.items[uid]
	return s, ok, nil
}

// Unbind 解绑玩家连接。
//
// 只有传入的 connID 等于当前记录时才会删除，避免旧连接断开时误删新连接。
func (m *MemoryManager) Unbind(ctx context.Context, uid, connID string) error {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.items[uid]
	if ok && (connID == "" || s.ConnID == connID) {
		delete(m.items, uid)
	}
	return nil
}

// Count 返回当前在线会话数量。
func (m *MemoryManager) Count(ctx context.Context) (int, error) {
	_ = ctx
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items), nil
}
