package session

import (
	"context"
	"sync"
)

// Store 是会话持久化接口。
//
// 后续如果需要把在线会话索引放到 Redis，可以实现这个接口替换内存版本。
type Store interface {
	Save(ctx context.Context, s Session) error
	Delete(ctx context.Context, uid string) error
	GetByUID(ctx context.Context, uid string) (Session, bool, error)
}

// LastServerStore 保存玩家最近一次被分配到的 GameServer。
//
// 登录重连时 NodeAllocator 读取这个索引，优先把玩家分配回原服以恢复本机内存热状态。
type LastServerStore interface {
	SaveLastServerID(ctx context.Context, uid string, serverID string) error
	GetLastServerID(ctx context.Context, uid string) (serverID string, ok bool, err error)
	DeleteLastServerID(ctx context.Context, uid string) error
}

// MemoryLastServerStore 是 LastServerStore 的内存实现。
//
// MVP 单进程可直接使用；多 GameServer 部署时替换为 Redis/DB 实现。
type MemoryLastServerStore struct {
	mu    sync.RWMutex
	items map[string]string
}

// NewMemoryLastServerStore 创建内存版最近 GameServer 索引。
func NewMemoryLastServerStore() *MemoryLastServerStore {
	return &MemoryLastServerStore{items: map[string]string{}}
}

// SaveLastServerID 记录玩家最近一次被分配到的 GameServer。
func (s *MemoryLastServerStore) SaveLastServerID(ctx context.Context, uid string, serverID string) error {
	_ = ctx
	if uid == "" || serverID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[uid] = serverID
	return nil
}

// GetLastServerID 查询玩家最近一次所在的 GameServer。
func (s *MemoryLastServerStore) GetLastServerID(ctx context.Context, uid string) (string, bool, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	serverID, ok := s.items[uid]
	return serverID, ok, nil
}

// DeleteLastServerID 删除玩家最近 GameServer 索引。
func (s *MemoryLastServerStore) DeleteLastServerID(ctx context.Context, uid string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, uid)
	return nil
}
