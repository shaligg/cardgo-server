package ws

import "errors"

// 定义常见的错误类型
var (
	ErrMissingUserID   = errors.New("missing user_id in auth message")
	ErrUnauthorized    = errors.New("unauthorized client")
	ErrInvalidMessage  = errors.New("invalid message format")
	ErrSessionNotFound = errors.New("session not found")
	ErrServerClosed    = errors.New("websocket server closed")
)
