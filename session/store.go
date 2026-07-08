package session

// Session 会话接口
type Session interface {
	GetID() string                           // 获取会话ID
	GetUserID() string                       // 获取用户ID
	Get(key string) (interface{}, error)     // 获取会话数据
	Set(key string, value interface{}) error // 设置会话数据
	Delete(key string) error                 // 删除会话数据
	Expire(duration int64) error             // 设置过期时间（秒）
	Close() error                            // 关闭会话
}

// Store 会话存储接口
type Store interface {
	Create(userID string) (Session, error)      // 创建会话
	Get(sessionID string) (Session, error)      // 获取会话
	GetByUserID(userID string) (Session, error) // 根据用户ID获取会话
	Delete(sessionID string) error              // 删除会话
	Cleanup() error                             // 清理过期会话
}
