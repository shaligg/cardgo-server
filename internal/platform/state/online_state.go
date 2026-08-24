package state

import (
	"sync"
	"time"
)

// PlayerState 是当前 GameServer 保存的玩家运行时展示快照。
type PlayerState struct {
	UID       string
	Version   int64
	Data      map[string]interface{}
	ExpiresAt time.Time
}

// OnlineState 管理在线玩家以及短时断线玩家的本机热状态。
type OnlineState struct {
	mu    sync.RWMutex
	items map[string]PlayerState
}

func NewOnlineState() *OnlineState {
	return &OnlineState{items: make(map[string]PlayerState)}
}

func (s *OnlineState) Get(uid string) (PlayerState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.items[uid]
	if ok && !v.ExpiresAt.IsZero() && !time.Now().Before(v.ExpiresAt) {
		delete(s.items, uid)
		return PlayerState{}, false
	}
	return v, ok
}

// Set 写入在线玩家的最新热状态，并清除之前的离线过期时间。
func (s *OnlineState) Set(st PlayerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.ExpiresAt = time.Time{}
	s.items[st.UID] = st
}

// MarkOnline 将已恢复连接的玩家重新标记为在线。
func (s *OnlineState) MarkOnline(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.items[uid]
	if !ok {
		return
	}
	st.ExpiresAt = time.Time{}
	s.items[uid] = st
}

// MarkOffline 保留断线玩家的热状态一段时间，供重连到原节点时恢复。
func (s *OnlineState) MarkOffline(uid string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.items[uid]
	if !ok {
		return
	}
	st.ExpiresAt = time.Now().Add(ttl)
	s.items[uid] = st
}

// DeleteExpired 删除已经超过离线保留时间的玩家状态。
func (s *OnlineState) DeleteExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := 0
	for uid, st := range s.items {
		if st.ExpiresAt.IsZero() || now.Before(st.ExpiresAt) {
			continue
		}
		delete(s.items, uid)
		deleted++
	}
	return deleted
}

// Len 返回当前保留的玩家状态数量。
func (s *OnlineState) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// List 返回当前节点保留的玩家状态元数据副本，供归属校验批量扫描。
func (s *OnlineState) List() []PlayerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PlayerState, 0, len(s.items))
	for _, st := range s.items {
		out = append(out, st)
	}
	return out
}

func (s *OnlineState) Delete(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, uid)
}
