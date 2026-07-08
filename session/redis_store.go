package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/redis"
	"github.com/google/uuid"
)

// RedisStore Redis会话存储实现
type RedisStore struct {
	ctx        context.Context // 上下文
	nodeID     string        // 节点ID
	expireTime int64         // 会话过期时间（秒）
}

// NewRedisStore 创建新的Redis会话存储
func NewRedisStore(nodeID string, expireTime int64) *RedisStore {
	return &RedisStore{
		ctx:        context.Background(),
		nodeID:     nodeID,
		expireTime: expireTime,
	}
}

// Create 创建新会话
func (s *RedisStore) Create(userID string) (Session, error) {
	// 生成会话ID
	sessionID := s.generateSessionID()

	// 当前时间
	now := time.Now()

	// 过期时间
	expireAt := now.Add(time.Duration(s.expireTime) * time.Second)

	// 初始化会话数据
	session := &sessionImpl{
		id:        sessionID,
		userID:    userID,
		data:      make(map[string]interface{}),
		createdAt: now,
		updatedAt: now,
		expiresAt: expireAt,
		store:     s,
		isModified: true,
	}

	// 保存会话元数据到Redis
	sessionData := SessionData{
		SessionID: sessionID,
		UserID:    userID,
		NodeID:    s.nodeID,
		Data:      session.data,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expireAt,
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return nil, err
	}

	// 获取Redis客户端
	client := redis.GetClient()

	// 保存会话数据
	if err := client.Set(s.ctx, fmt.Sprintf("session:%s", sessionID), data, time.Duration(s.expireTime)*time.Second).Err(); err != nil {
		return nil, err
	}

	// 保存用户ID到会话ID的映射
	if err := client.Set(s.ctx, fmt.Sprintf("user_session:%s", userID), sessionID, time.Duration(s.expireTime)*time.Second).Err(); err != nil {
		return nil, err
	}

	return session, nil
}

// Get 获取会话
func (s *RedisStore) Get(sessionID string) (Session, error) {
	// 获取Redis客户端
	client := redis.GetClient()

	// 从Redis获取会话数据
	data, err := client.Get(s.ctx, fmt.Sprintf("session:%s", sessionID)).Result()
	if err != nil {
		return nil, err
	}

	// 解析会话数据
	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, err
	}

	// 创建会话对象
	session := &sessionImpl{
		id:        sessionID,
		userID:    sessionData.UserID,
		data:      sessionData.Data,
		createdAt: sessionData.CreatedAt,
		updatedAt: sessionData.UpdatedAt,
		expiresAt: sessionData.ExpiresAt,
		store:     s,
		isModified: false,
	}

	return session, nil
}

// GetByUserID 根据用户ID获取会话
func (s *RedisStore) GetByUserID(userID string) (Session, error) {
	// 获取Redis客户端
	client := redis.GetClient()

	// 获取用户对应的会话ID
	sessionID, err := client.Get(s.ctx, fmt.Sprintf("user_session:%s", userID)).Result()
	if err != nil {
		return nil, err
	}

	// 根据会话ID获取会话
	return s.Get(sessionID)
}

// Delete 删除会话
func (s *RedisStore) Delete(sessionID string) error {
	// 获取会话数据，以便获取用户ID
	session, err := s.Get(sessionID)
	if err != nil {
		return err
	}

	// 获取Redis客户端
	client := redis.GetClient()

	// 删除会话数据
	if err := client.Del(s.ctx, fmt.Sprintf("session:%s", sessionID)).Err(); err != nil {
		return err
	}

	// 删除用户ID到会话ID的映射
	if err := client.Del(s.ctx, fmt.Sprintf("user_session:%s", session.GetUserID())).Err(); err != nil {
		return err
	}

	return nil
}

// Cleanup 清理过期会话
func (s *RedisStore) Cleanup() error {
	// Redis会自动清理过期键，所以这里不需要额外的清理逻辑
	// 如果需要更复杂的清理逻辑，可以使用SCAN命令遍历所有会话键，检查过期时间
	return nil
}

// generateSessionID 生成会话ID
func (s *RedisStore) generateSessionID() string {
	// 使用UUID生成会话ID
	return fmt.Sprintf("session_%s", uuid.New().String())
}
