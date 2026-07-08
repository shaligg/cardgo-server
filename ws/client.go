package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client WebSocket客户端连接
type Client struct {
	conn      *websocket.Conn    // WebSocket连接
	server    *Server            // 所属服务器
	userID    string             // 用户ID
	sessionID string             // 会话ID
	mutex     sync.Mutex         // 并发锁，保护conn的写入
	sendChan  chan *Message      // 发送消息的通道
	ctx       context.Context    // 上下文
	cancel    context.CancelFunc // 取消函数
}

// NewClient 创建新的客户端连接
func NewClient(conn *websocket.Conn, server *Server) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		conn:     conn,
		server:   server,
		sendChan: make(chan *Message, 100), // 缓冲区大小为100
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetUserID 设置用户ID
func (c *Client) SetUserID(userID string) {
	c.userID = userID
}

// GetUserID 获取用户ID
func (c *Client) GetUserID() string {
	return c.userID
}

// SetSessionID 设置会话ID
func (c *Client) SetSessionID(sessionID string) {
	c.sessionID = sessionID
}

// GetSessionID 获取会话ID
func (c *Client) GetSessionID() string {
	return c.sessionID
}

// Send 发送消息
func (c *Client) Send(msg *Message) error {
	select {
	case c.sendChan <- msg:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		// 通道已满，直接发送
		return c.writeMessage(msg)
	}
}

// Receive 接收消息
func (c *Client) Receive() (*Message, error) {
	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	c.cancel()
	close(c.sendChan)
	return c.conn.Close()
}

// Start 启动客户端的读写循环
func (c *Client) Start() {
	// 启动读循环
	go c.readLoop()
	// 启动写循环
	go c.writeLoop()
}

// readLoop 读取消息循环
func (c *Client) readLoop() {
	defer func() {
		c.server.removeClient(c)
		c.Close()
	}()

	// 设置读取超时
	c.conn.SetReadDeadline(time.Now().Add(c.server.config.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.server.config.PongWait))
		return nil
	})

	for {
		msg, err := c.Receive()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		// 处理消息
		if err := c.server.handleMessage(c, msg); err != nil {
			log.Printf("Error handling message: %v", err)
			// 发送错误消息给客户端
			errMsg, _ := NewMessage(MessageTypeError, "", map[string]string{"error": err.Error()})
			c.Send(errMsg)
		}
	}
}

// writeLoop 写入消息循环
func (c *Client) writeLoop() {
	// 心跳定时器
	heartbeatTicker := time.NewTicker(c.server.config.HeartbeatInterval)
	defer func() {
		heartbeatTicker.Stop()
		c.server.removeClient(c)
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.sendChan:
			if !ok {
				// 通道已关闭
				return
			}

			if err := c.writeMessage(msg); err != nil {
				log.Printf("Error writing message: %v", err)
				return
			}

		case <-heartbeatTicker.C:
			// 发送心跳消息
			heartbeatMsg, _ := NewMessage(MessageTypeHeartbeat, "", map[string]string{"status": "ok"})
			if err := c.writeMessage(heartbeatMsg); err != nil {
				log.Printf("Error sending heartbeat: %v", err)
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// writeMessage 写入消息到WebSocket连接
func (c *Client) writeMessage(msg *Message) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 设置写入超时
	c.conn.SetWriteDeadline(time.Now().Add(c.server.config.WriteWait))

	// 序列化消息
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 写入消息
	return c.conn.WriteMessage(websocket.TextMessage, msgBytes)
}
