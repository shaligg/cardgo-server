package ws

import (
	"encoding/json"
	"time"
)

// Message WebSocket消息结构
type Message struct {
	Type      string          `json:"type"`      // 消息类型
	UserID    string          `json:"user_id"`   // 发送者ID
	Content   json.RawMessage `json:"content"`   // 消息内容
	Timestamp int64           `json:"timestamp"` // 时间戳
}

// NewMessage 创建新消息
func NewMessage(msgType string, userID string, content interface{}) (*Message, error) {
	// 将内容转换为JSON
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:      msgType,
		UserID:    userID,
		Content:   contentBytes,
		Timestamp: time.Now().Unix(),
	}, nil
}

// MessageType 消息类型常量
const (
	MessageTypeText      = "text"      // 文本消息
	MessageTypeHeartbeat = "heartbeat" // 心跳消息
	MessageTypeError     = "error"     // 错误消息
	MessageTypeSystem    = "system"    // 系统消息
	MessageTypeAuth      = "auth"      // 认证消息
)
