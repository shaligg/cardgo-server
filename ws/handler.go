package ws

import "encoding/json"

// Handler 消息处理器接口
type Handler interface {
	Handle(client *Client, msg *Message) error // 处理消息
}

// HandlerFunc 函数类型的消息处理器
type HandlerFunc func(client *Client, msg *Message) error

// Handle 实现Handler接口
func (f HandlerFunc) Handle(client *Client, msg *Message) error {
	return f(client, msg)
}

// DefaultHandler 默认消息处理器
type DefaultHandler struct{}

// Handle 处理默认消息
func (h *DefaultHandler) Handle(client *Client, msg *Message) error {
	// 处理心跳消息
	if msg.Type == MessageTypeHeartbeat {
		// 心跳消息，无需特殊处理，只需要回复即可
		heartbeatMsg, _ := NewMessage(MessageTypeHeartbeat, "", map[string]string{"status": "ok"})
		return client.Send(heartbeatMsg)
	}

	// 处理认证消息
	if msg.Type == MessageTypeAuth {
		return h.handleAuth(client, msg)
	}

	// 处理文本消息
	if msg.Type == MessageTypeText {
		return h.handleText(client, msg)
	}

	return nil
}

// handleAuth 处理认证消息
func (h *DefaultHandler) handleAuth(client *Client, msg *Message) error {
	// 解析认证信息
	var authData map[string]string
	if err := parseMessageContent(msg, &authData); err != nil {
		return err
	}

	// 获取用户ID
	userID, ok := authData["user_id"]
	if !ok {
		return ErrMissingUserID
	}

	// 设置客户端的用户ID
	client.SetUserID(userID)

	// 创建会话
	session, err := client.server.sessionStore.Create(userID)
	if err != nil {
		return err
	}

	// 设置客户端的会话ID
	client.SetSessionID(session.GetID())

	// 发送认证成功消息
	authOkMsg, _ := NewMessage(MessageTypeSystem, "", map[string]string{
		"message":    "Authentication successful",
		"session_id": session.GetID(),
		"user_id":    userID,
	})

	return client.Send(authOkMsg)
}

// handleText 处理文本消息
func (h *DefaultHandler) handleText(client *Client, msg *Message) error {
	// 检查用户是否已认证
	if client.GetUserID() == "" {
		return ErrUnauthorized
	}

	// 广播消息给所有客户端
	return client.server.Broadcast(msg)
}

// parseMessageContent 解析消息内容
func parseMessageContent(msg *Message, v interface{}) error {
	return unmarshalJSON(msg.Content, v)
}

// unmarshalJSON 解析JSON数据
func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
