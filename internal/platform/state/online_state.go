package state

import "sync"

type PlayerState struct {
	UID     string
	Version int64
	Data    map[string]interface{}
}

type OnlineState struct {
	mu    sync.RWMutex
	items map[string]PlayerState
}

func NewOnlineState() *OnlineState {
	return &OnlineState{items: make(map[string]PlayerState)}
}

func (s *OnlineState) Get(uid string) (PlayerState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[uid]
	return v, ok
}

func (s *OnlineState) Set(st PlayerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[st.UID] = st
}

func (s *OnlineState) Delete(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, uid)
}
