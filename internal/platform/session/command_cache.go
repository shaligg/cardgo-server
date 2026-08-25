package session

import (
	"encoding/json"
	"sync"
	"time"
)

// CommandResult 是一次成功业务请求的近期结果。
type CommandResult struct {
	OpCode      int32
	PayloadHash string
	Result      json.RawMessage
}

type playerCommandCache struct {
	expiresAt time.Time
	order     []string
	entries   map[string]CommandResult
}

// CommandCache 按玩家保存少量近期业务结果，用于处理网络超时重试。
type CommandCache struct {
	mu         sync.Mutex
	players    map[string]*playerCommandCache
	ttl        time.Duration
	maxEntries int
	maxBytes   int
}

// NewCommandCache 创建进程内近期请求缓存。
func NewCommandCache(ttl time.Duration, maxEntries int, maxBytes int) *CommandCache {
	return &CommandCache{
		players:    make(map[string]*playerCommandCache),
		ttl:        ttl,
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
	}
}

// Get 返回玩家指定请求的首次成功结果。
func (c *CommandCache) Get(uid string, reqID string) (CommandResult, bool) {
	if c == nil || uid == "" || reqID == "" {
		return CommandResult{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	player := c.players[uid]
	if player == nil {
		return CommandResult{}, false
	}
	if time.Now().After(player.expiresAt) {
		delete(c.players, uid)
		return CommandResult{}, false
	}
	result, ok := player.entries[reqID]
	if !ok {
		return CommandResult{}, false
	}
	result.Result = append(json.RawMessage(nil), result.Result...)
	return result, true
}

// Put 保存成功结果；过大的响应不进入近期缓存。
func (c *CommandCache) Put(uid string, reqID string, result CommandResult) bool {
	if c == nil || uid == "" || reqID == "" || c.ttl <= 0 || c.maxEntries <= 0 || len(result.Result) > c.maxBytes {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	player := c.players[uid]
	if player == nil || time.Now().After(player.expiresAt) {
		player = &playerCommandCache{entries: make(map[string]CommandResult)}
		c.players[uid] = player
	}
	if _, exists := player.entries[reqID]; !exists {
		player.order = append(player.order, reqID)
	}
	result.Result = append(json.RawMessage(nil), result.Result...)
	player.entries[reqID] = result
	player.expiresAt = time.Now().Add(c.ttl)
	for len(player.order) > c.maxEntries {
		oldest := player.order[0]
		player.order = player.order[1:]
		delete(player.entries, oldest)
	}
	return true
}

// Delete 删除玩家迁移后不再可信的本机结果。
func (c *CommandCache) Delete(uid string) {
	if c == nil || uid == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.players, uid)
}

// CleanupExpired 清理超过离线恢复窗口的玩家结果。
func (c *CommandCache) CleanupExpired(now time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for uid, player := range c.players {
		if now.After(player.expiresAt) {
			delete(c.players, uid)
		}
	}
}
