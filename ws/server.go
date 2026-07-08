package ws

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/bigfish/go_orm_1/config"
	"github.com/bigfish/go_orm_1/session"
	"github.com/gorilla/websocket"
)

// Server WebSocket服务器
type Server struct {
	config        config.WebSocketConfig // WebSocket配置
	listener      *http.Server           // HTTP服务器
	upgrader      websocket.Upgrader     // WebSocket升级器
	clients       map[*Client]bool       // 客户端连接映射
	clientsMutex  sync.RWMutex           // 客户端映射的互斥锁
	handlers      map[string]Handler     // 消息处理器映射
	handlersMutex sync.RWMutex           // 处理器映射的互斥锁
	sessionStore  session.Store          // 会话存储
	ctx           context.Context        // 上下文
	cancel        context.CancelFunc     // 取消函数
	isRunning     bool                   // 服务器是否正在运行
}

// NewServer 创建新的WebSocket服务器
func NewServer() *Server {
	// 获取配置
	cfg := config.GetConfig().WebSocket

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建WebSocket升级器
	upgrader := websocket.Upgrader{
		ReadBufferSize:  cfg.ReadBufferSize,
		WriteBufferSize: cfg.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			// 允许所有来源，生产环境中应该限制
			return true
		},
	}

	// 创建会话存储
	sessionStore := session.NewRedisStore(cfg.NodeID, cfg.SessionExpire)

	// 创建服务器实例
	server := &Server{
		config:       cfg,
		upgrader:     upgrader,
		clients:      make(map[*Client]bool),
		handlers:     make(map[string]Handler),
		sessionStore: sessionStore,
		ctx:          ctx,
		cancel:       cancel,
		isRunning:    false,
	}

	// 注册默认处理器
	server.RegisterHandler("", &DefaultHandler{})

	return server
}

// Start 启动WebSocket服务器
func (s *Server) Start() error {
	if s.isRunning {
		return nil
	}

	// 创建HTTP服务器
	s.listener = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: s,
	}

	// 标记服务器为运行中
	s.isRunning = true

	// 启动服务器
	go func() {
		log.Printf("WebSocket server starting on port %d", s.config.Port)
		if err := s.listener.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting WebSocket server: %v", err)
			// 服务器启动失败，标记为未运行
			s.isRunning = false
			s.cancel()
		}
	}()

	return nil
}

// Stop 停止WebSocket服务器
func (s *Server) Stop() error {
	if !s.isRunning {
		return nil
	}

	// 标记服务器为未运行
	s.isRunning = false

	// 取消上下文
	s.cancel()

	// 关闭所有客户端连接
	s.clientsMutex.Lock()
	for client := range s.clients {
		client.Close()
	}
	s.clients = make(map[*Client]bool)
	s.clientsMutex.Unlock()

	// 关闭HTTP服务器
	if s.listener != nil {
		log.Printf("Stopping WebSocket server on port %d", s.config.Port)
		return s.listener.Shutdown(context.Background())
	}

	return nil
}

// ServeHTTP 处理HTTP请求
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}

	// 检查连接数是否超过限制
	s.clientsMutex.RLock()
	if len(s.clients) >= s.config.MaxConnections {
		s.clientsMutex.RUnlock()
		log.Printf("WebSocket connection limit reached: %d/%d", len(s.clients), s.config.MaxConnections)
		conn.Close()
		return
	}
	s.clientsMutex.RUnlock()

	// 创建客户端连接
	client := NewClient(conn, s)

	// 添加客户端到映射
	s.clientsMutex.Lock()
	s.clients[client] = true
	s.clientsMutex.Unlock()

	// 启动客户端的读写循环
	client.Start()

	log.Printf("New WebSocket connection established. Total clients: %d", s.GetClientCount())
}

// Broadcast 广播消息给所有客户端
func (s *Server) Broadcast(msg *Message) error {
	// 发送消息给所有客户端
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	for client := range s.clients {
		// 使用通道发送消息
		select {
		case client.sendChan <- msg:
			// 消息已发送到通道
		default:
			// 通道已满，直接发送
			if err := client.writeMessage(msg); err != nil {
				log.Printf("Error broadcasting message to client: %v", err)
				// 关闭客户端连接
				s.removeClient(client)
				client.Close()
			}
		}
	}

	return nil
}

// GetClientCount 获取当前连接的客户端数量
func (s *Server) GetClientCount() int {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	return len(s.clients)
}

// RegisterHandler 注册消息处理器
func (s *Server) RegisterHandler(msgType string, handler Handler) {
	s.handlersMutex.Lock()
	defer s.handlersMutex.Unlock()

	s.handlers[msgType] = handler
}

// removeClient 移除客户端连接
func (s *Server) removeClient(client *Client) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	if _, ok := s.clients[client]; ok {
		delete(s.clients, client)
		log.Printf("WebSocket connection closed. Total clients: %d", len(s.clients))
	}
}

// handleMessage 处理消息
func (s *Server) handleMessage(client *Client, msg *Message) error {
	// 获取消息处理器
	s.handlersMutex.RLock()
	handler, ok := s.handlers[msg.Type]
	if !ok {
		// 没有找到特定类型的处理器，使用默认处理器
		handler, ok = s.handlers[""]
		if !ok {
			s.handlersMutex.RUnlock()
			return ErrInvalidMessage
		}
	}
	s.handlersMutex.RUnlock()

	// 调用处理器处理消息
	return handler.Handle(client, msg)
}
