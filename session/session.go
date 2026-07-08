package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/redis"
)

// SessionData 会话数据结构
type SessionData struct {
	SessionID string                 `json:"session_id"` // 会话ID
	UserID    string                 `json:"user_id"`    // 用户ID
	NodeID    string                 `json:"node_id"`    // 节点ID
	Data      map[string]interface{} `json:"data"`       // 会话数据
	CreatedAt time.Time              `json:"created_at"` // 创建时间
	UpdatedAt time.Time              `json:"updated_at"` // 更新时间
	ExpiresAt time.Time              `json:"expires_at"` // 过期时间
}

// sessionImpl Session接口的具体实现
type sessionImpl struct {
	id         string                 // 会话ID
	userID     string                 // 用户ID
	data       map[string]interface{} // 会话数据
	createdAt  time.Time              // 创建时间
	updatedAt  time.Time              // 更新时间
	expiresAt  time.Time              // 过期时间
	store      *RedisStore            // 所属的存储
	isModified bool                   // 数据是否被修改
}

// NewSession 创建新的会话
type NewSession struct {
	UserID string // 用户ID
	NodeID string // 节点ID
}

// GetID 获取会话ID
func (s *sessionImpl) GetID() string {
	return s.id
}

// GetUserID 获取用户ID
func (s *sessionImpl) GetUserID() string {
	return s.userID
}

// Get 获取会话数据
func (s *sessionImpl) Get(key string) (interface{}, error) {
	// 如果数据已经在内存中，直接返回
	if val, ok := s.data[key]; ok {
		return val, nil
	}

	// 获取Redis客户端
	client := redis.GetClient()

	// 从Redis中获取
	val, err := client.HGet(s.store.ctx, s.id, key).Result()
	if err != nil {
		return nil, err
	}

	// 解析数据
	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, err
	}

	// 缓存到内存中
	s.data[key] = result
	return result, nil
}

// Set 设置会话数据
func (s *sessionImpl) Set(key string, value interface{}) error {
	// 更新内存中的数据
	s.data[key] = value
	s.isModified = true
	s.updatedAt = time.Now()

	// 序列化数据
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// 获取Redis客户端
	client := redis.GetClient()

	// 保存到Redis
	return client.HSet(s.store.ctx, s.id, key, data).Err()
}

// Delete 删除会话数据
func (s *sessionImpl) Delete(key string) error {
	// 删除内存中的数据
	delete(s.data, key)
	s.isModified = true
	s.updatedAt = time.Now()

	// 获取Redis客户端
	client := redis.GetClient()

	// 从Redis中删除
	return client.HDel(s.store.ctx, s.id, key).Err()
}

// Expire 设置过期时间（秒）
func (s *sessionImpl) Expire(duration int64) error {
	// 更新过期时间
	s.expiresAt = time.Now().Add(time.Duration(duration) * time.Second)

	// 获取Redis客户端
	client := redis.GetClient()

	// 设置Redis中的过期时间
	return client.Expire(s.store.ctx, s.id, time.Duration(duration)*time.Second).Err()
}

// Close 关闭会话
func (s *sessionImpl) Close() error {
	// 如果数据被修改，保存到Redis
	if s.isModified {
		// 保存会话元数据
		sessionData := SessionData{
			SessionID: s.id,
			UserID:    s.userID,
			Data:      s.data,
			CreatedAt: s.createdAt,
			UpdatedAt: s.updatedAt,
			ExpiresAt: s.expiresAt,
		}

		// 序列化会话数据
		data, err := json.Marshal(sessionData)
		if err != nil {
			return err
		}

		// 获取Redis客户端
		client := redis.GetClient()

		// 保存到Redis
		return client.Set(s.store.ctx, fmt.Sprintf("session:%s", s.id), data, s.expiresAt.Sub(time.Now())).Err()
	}

	return nil
}
