package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	dto "github.com/bigfish/go_orm_1/internal/framework/transport/dto"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Options struct {
	NodeID          string
	Addr            string
	MaxConnections  int
	DrainMode       bool
	Verifier        auth.TicketVerifier
	SessionManager  session.Manager
	Heartbeat       HeartbeatConfig
	SendQueueSize   int
	InboundMinGap   time.Duration
	MaxMessageBytes int64
	AllowedOrigins  []string
	Codec           EnvelopeCodec
	BizHandler      BizHandler
	OnSessionBound  func(ctx context.Context, uid string, connID string) error
	OnDisconnect    func(ctx context.Context, uid string, connID string)
	OnRestoreState  func(ctx context.Context, uid string) (map[string]interface{}, bool)
	Metrics         *metrics.Registry
}

// BizHandler 是 WS 层调用业务协议分发器的抽象入口。
//
// WS 层只关心连接、鉴权和收发包，不直接依赖玩家、背包、关卡等具体业务模块。
type BizHandler interface {
	Handle(ctx context.Context, uid string, opCode int32, payload json.RawMessage) (interface{}, *terrors.BizError)
}

type Server struct {
	Addr           string
	NodeID         string
	MaxConnections int

	verifier       auth.TicketVerifier
	sessionManager session.Manager
	heartbeat      HeartbeatConfig
	bizHandler     BizHandler
	onSessionBound func(ctx context.Context, uid string, connID string) error
	onDisconnect   func(ctx context.Context, uid string, connID string)
	onRestoreState func(ctx context.Context, uid string) (map[string]interface{}, bool)
	metrics        *metrics.Registry
	codec          EnvelopeCodec

	sendQueueSize   int
	inboundLimiter  *RateLimiter
	maxMessageBytes int64

	upgrader websocket.Upgrader
	httpSrv  *http.Server

	drainMode   atomic.Bool
	connections atomic.Int64

	mu      sync.RWMutex
	clients map[string]*Client
	sockets map[*websocket.Conn]struct{}

	handlerMu sync.Mutex
	stopping  bool
	handlerWG sync.WaitGroup
}

func NewServer(opts Options) *Server {
	if opts.MaxConnections <= 0 {
		opts.MaxConnections = 2000
	}
	if opts.Heartbeat.PongWait <= 0 {
		opts.Heartbeat.PongWait = 60 * time.Second
	}
	if opts.Heartbeat.WriteWait <= 0 {
		opts.Heartbeat.WriteWait = 10 * time.Second
	}
	if opts.SendQueueSize <= 0 {
		opts.SendQueueSize = 256
	}
	if opts.MaxMessageBytes <= 0 {
		opts.MaxMessageBytes = 64 * 1024
	}
	if opts.Codec == nil {
		opts.Codec = JSONEnvelopeCodec{}
	}

	var limiter *RateLimiter
	if opts.InboundMinGap > 0 {
		limiter = NewRateLimiter(opts.InboundMinGap)
	}

	s := &Server{
		Addr:            opts.Addr,
		NodeID:          opts.NodeID,
		MaxConnections:  opts.MaxConnections,
		verifier:        opts.Verifier,
		sessionManager:  opts.SessionManager,
		heartbeat:       opts.Heartbeat,
		bizHandler:      opts.BizHandler,
		onSessionBound:  opts.OnSessionBound,
		onDisconnect:    opts.OnDisconnect,
		onRestoreState:  opts.OnRestoreState,
		metrics:         opts.Metrics,
		codec:           opts.Codec,
		sendQueueSize:   opts.SendQueueSize,
		inboundLimiter:  limiter,
		maxMessageBytes: opts.MaxMessageBytes,
		upgrader: websocket.Upgrader{
			CheckOrigin: buildOriginChecker(opts.AllowedOrigins),
		},
		clients: make(map[string]*Client),
		sockets: make(map[*websocket.Conn]struct{}),
	}
	s.drainMode.Store(opts.DrainMode)
	return s
}

// buildOriginChecker 允许原生客户端无 Origin 连接，浏览器连接必须命中配置白名单。
func buildOriginChecker(allowedOrigins []string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		if _, ok := allowed["*"]; ok {
			return true
		}
		_, ok := allowed[origin]
		return ok
	}
}

func (s *Server) Start(ctx context.Context) error {
	_ = ctx
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.IsDrainMode() {
			_, _ = w.Write([]byte(`{"ready":false,"drain_mode":true}`))
			return
		}
		_, _ = w.Write([]byte(`{"ready":true,"drain_mode":false}`))
	})
	s.httpSrv = &http.Server{
		Addr:    s.Addr,
		Handler: mux,
	}
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen ws %s: %w", s.Addr, err)
	}
	s.Addr = listener.Addr().String()
	s.httpSrv.Addr = s.Addr

	go func() {
		ilog.Infof("ws server listening on %s", s.Addr)
		if err := s.httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("ws server stopped with error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.SetDrainMode(true)
	s.handlerMu.Lock()
	s.stopping = true
	s.handlerMu.Unlock()

	var firstErr error
	if s.httpSrv != nil {
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	s.mu.Lock()
	for _, c := range s.clients {
		c.Close()
	}
	for conn := range s.sockets {
		_ = conn.Close()
	}
	s.clients = make(map[string]*Client)
	s.sockets = make(map[*websocket.Conn]struct{})
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.handlerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		return firstErr
	case <-ctx.Done():
		if firstErr != nil {
			return firstErr
		}
		return ctx.Err()
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.IsDrainMode() || !s.tryAcquireConnection() {
		s.writeServerFullHTTP(w)
		return
	}
	if !s.beginHandler() {
		s.connections.Add(-1)
		s.writeServerFullHTTP(w)
		return
	}
	defer s.handlerWG.Done()
	s.observeConnections()
	defer func() {
		s.connections.Add(-1)
		s.observeConnections()
	}()

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.addSocket(conn)
	defer s.removeSocket(conn)
	defer conn.Close()

	conn.SetReadLimit(s.maxMessageBytes)
	_ = conn.SetReadDeadline(time.Now().Add(s.heartbeat.PongWait))

	uid, connID, client, ok := s.authenticate(r.Context(), conn)
	if !ok {
		return
	}
	defer func() {
		_ = s.sessionManager.Unbind(context.Background(), uid, connID)
		if s.inboundLimiter != nil {
			s.inboundLimiter.Delete(connID + ":biz")
		}
		s.removeClient(connID)
		if s.onDisconnect != nil {
			s.onDisconnect(context.Background(), uid, connID)
		}
		client.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		req, err := s.codec.DecodeEnvelope(data)
		if err != nil {
			if !s.sendError(client, 0, terrors.CodeBadRequest, "invalid envelope") {
				return
			}
			continue
		}
		// 任意合法协议都代表连接仍然活跃；空闲连接由客户端业务心跳续期。
		_ = conn.SetReadDeadline(time.Now().Add(s.heartbeat.PongWait))

		switch req.Type {
		case dto.TypeHeartbeatReq:
			if !s.enqueueEnvelope(client, dto.Envelope{
				Seq:  req.Seq,
				Type: dto.TypeHeartbeatAck,
				TS:   time.Now().Unix(),
				Payload: map[string]interface{}{
					"ok": true,
				},
			}) {
				return
			}
		case dto.TypeBizReq:
			if req.OpCode == 0 {
				if !s.sendError(client, req.Seq, terrors.CodeBadRequest, "missing op_code") {
					return
				}
				continue
			}
			if s.inboundLimiter != nil && !s.inboundLimiter.Allow(client.ConnID+":biz") {
				if s.metrics != nil {
					s.metrics.IncWSRateLimited()
				}
				if !s.sendError(client, req.Seq, terrors.CodeRateLimited, "too many requests") {
					return
				}
				continue
			}
			if s.bizHandler == nil {
				if !s.enqueueEnvelope(client, dto.Envelope{
					Seq:  req.Seq,
					Type: dto.TypeBizAck,
					TS:   time.Now().Unix(),
					Payload: map[string]interface{}{
						"ok":  true,
						"uid": uid,
					},
				}) {
					return
				}
				continue
			}
			startedAt := time.Now()
			resp, bizErr := s.bizHandler.Handle(r.Context(), uid, req.OpCode, req.Payload)
			if s.metrics != nil {
				s.metrics.ObserveWSBizDuration(time.Since(startedAt))
			}
			if bizErr != nil {
				if !s.sendError(client, req.Seq, bizErr.Code, bizErr.Msg) {
					return
				}
				continue
			}
			if !s.enqueueEnvelope(client, dto.Envelope{
				Seq:    req.Seq,
				Type:   dto.TypeBizAck,
				OpCode: req.OpCode,
				TS:     time.Now().Unix(),
				Payload: map[string]interface{}{
					"ok":   true,
					"data": resp,
				},
			}) {
				return
			}
		default:
			if !s.sendError(client, req.Seq, terrors.CodeBadRequest, "unsupported message type") {
				return
			}
		}
	}
}

func (s *Server) authenticate(ctx context.Context, conn *websocket.Conn) (uid string, connID string, client *Client, ok bool) {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, firstData, err := conn.ReadMessage()
	if err != nil {
		return "", "", nil, false
	}

	first, err := s.codec.DecodeEnvelope(firstData)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, 0, terrors.CodeBadRequest, "invalid envelope")
		return "", "", nil, false
	}
	if first.Type != dto.TypeAuthReq {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, first.Seq, terrors.CodeAuthInvalid, "first frame must be auth_req")
		return "", "", nil, false
	}

	var authReq dto.AuthReqPayload
	if len(first.Payload) == 0 || json.Unmarshal(first.Payload, &authReq) != nil || authReq.Ticket == "" {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, first.Seq, terrors.CodeBadRequest, "missing ticket")
		return "", "", nil, false
	}

	claims, err := s.verifier.Verify(ctx, authReq.Ticket, s.NodeID, time.Now().Unix())
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		code := terrors.CodeAuthInvalid
		switch {
		case errors.Is(err, auth.ErrExpiredToken):
			code = terrors.CodeAuthExpired
		case errors.Is(err, auth.ErrReplay):
			code = terrors.CodeAuthReplay
		}
		_ = s.writeErrorConn(conn, first.Seq, code, err.Error())
		return "", "", nil, false
	}

	if s.IsDrainMode() {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeServerFullWS(conn, first.Seq)
		return "", "", nil, false
	}

	connID = uuid.NewString()
	oldConnID, accepted, err := s.sessionManager.BindWithinLimit(ctx, session.Session{
		UID:      claims.UID,
		ConnID:   connID,
		LoginAt:  time.Now(),
		LastSeen: time.Now(),
		ClientIP: "",
	}, s.MaxConnections)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, first.Seq, terrors.CodeInternal, "session bind failed")
		return "", "", nil, false
	}
	if !accepted {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeServerFullWS(conn, first.Seq)
		return "", "", nil, false
	}
	if s.onSessionBound != nil {
		if err := s.onSessionBound(ctx, claims.UID, connID); err != nil {
			_ = s.sessionManager.Unbind(context.Background(), claims.UID, connID)
			if s.metrics != nil {
				s.metrics.IncWSAuthFailed()
			}
			_ = s.writeErrorConn(conn, first.Seq, terrors.CodeInternal, "claim player owner failed")
			return "", "", nil, false
		}
	}

	client = NewClient(connID, claims.UID, conn, s.sendQueueSize, s.heartbeat.WriteWait, s.codec)
	s.addClient(client)
	go client.RunWritePump()

	ack := dto.Envelope{
		Seq:  first.Seq,
		Type: dto.TypeAuthAck,
		TS:   time.Now().Unix(),
		Payload: dto.AuthAckPayload{
			OK:        true,
			UID:       claims.UID,
			SessionID: connID,
		},
	}
	if s.onRestoreState != nil {
		if resync, ok := s.onRestoreState(ctx, claims.UID); ok {
			payload := ack.Payload.(dto.AuthAckPayload)
			payload.Resync = resync
			ack.Payload = payload
		}
	}
	if !s.enqueueEnvelope(client, ack) {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		s.removeClient(connID)
		_ = s.sessionManager.Unbind(context.Background(), claims.UID, connID)
		client.Close()
		return "", "", nil, false
	}
	if s.metrics != nil {
		s.metrics.IncWSAuthSuccess()
	}
	if oldConnID != "" && oldConnID != connID {
		s.kickClient(oldConnID, "replaced by new login")
	}

	_ = conn.SetReadDeadline(time.Now().Add(s.heartbeat.PongWait))
	return claims.UID, connID, client, true
}

func (s *Server) addClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c.ConnID] = c
}

func (s *Server) removeClient(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, connID)
}

func (s *Server) addSocket(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sockets[conn] = struct{}{}
}

func (s *Server) removeSocket(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sockets, conn)
}

func (s *Server) beginHandler() bool {
	s.handlerMu.Lock()
	defer s.handlerMu.Unlock()
	if s.stopping {
		return false
	}
	s.handlerWG.Add(1)
	return true
}

func (s *Server) kickClient(connID string, reason string) {
	s.mu.RLock()
	client := s.clients[connID]
	s.mu.RUnlock()
	if client == nil {
		return
	}

	env := dto.Envelope{
		Seq:  0,
		Type: dto.TypeKick,
		TS:   time.Now().Unix(),
		Payload: map[string]interface{}{
			"reason": reason,
		},
	}

	if client.TryEnqueue(env) {
		go func(c *Client) {
			time.Sleep(50 * time.Millisecond)
			c.Close()
		}(client)
		return
	}
	client.Close()
}

// KickUID 关闭当前节点中指定玩家的连接。
func (s *Server) KickUID(ctx context.Context, uid string, reason string) {
	current, ok, err := s.sessionManager.GetByUID(ctx, uid)
	if err != nil || !ok {
		return
	}
	s.kickClient(current.ConnID, reason)
}

// KickConnection 只关闭 UID 和连接 ID 都匹配的连接，避免延迟通知误踢新会话。
func (s *Server) KickConnection(uid string, connID string, reason string) bool {
	if uid == "" || connID == "" {
		return false
	}
	s.mu.RLock()
	client := s.clients[connID]
	s.mu.RUnlock()
	if client == nil || client.UID != uid {
		return false
	}
	s.kickClient(connID, reason)
	return true
}

// KickAll 向当前节点的所有客户端发送踢出通知并关闭连接。
func (s *Server) KickAll(reason string) int {
	s.mu.RLock()
	connIDs := make([]string, 0, len(s.clients))
	for connID := range s.clients {
		connIDs = append(connIDs, connID)
	}
	s.mu.RUnlock()
	for _, connID := range connIDs {
		s.kickClient(connID, reason)
	}
	return len(connIDs)
}

func (s *Server) writeServerFullHTTP(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"code":"SERVER_FULL","retry_after_sec":3,"candidates":[]}`))
}

func (s *Server) writeServerFullWS(conn *websocket.Conn, seq int64) error {
	return s.writeEnvelopeConn(conn, dto.Envelope{
		Seq:  seq,
		Type: dto.TypeServerFull,
		TS:   time.Now().Unix(),
		Payload: dto.ServerFullPayload{
			Code:          terrors.CodeServerFull,
			RetryAfterSec: 3,
			Candidates:    []interface{}{},
		},
	})
}

func (s *Server) writeErrorConn(conn *websocket.Conn, seq int64, code string, msg string) error {
	return s.writeEnvelopeConn(conn, dto.Envelope{
		Seq:  seq,
		Type: dto.TypeError,
		TS:   time.Now().Unix(),
		Payload: dto.ErrorPayload{
			Code: code,
			Msg:  msg,
		},
	})
}

func (s *Server) sendError(client *Client, seq int64, code string, msg string) bool {
	return s.enqueueEnvelope(client, dto.Envelope{
		Seq:  seq,
		Type: dto.TypeError,
		TS:   time.Now().Unix(),
		Payload: dto.ErrorPayload{
			Code: code,
			Msg:  msg,
		},
	})
}

func (s *Server) enqueueEnvelope(client *Client, env dto.Envelope) bool {
	if client.TryEnqueue(env) {
		return true
	}

	ilog.Errorf("send queue full uid=%s conn=%s type=%s", client.UID, client.ConnID, env.Type)
	if s.metrics != nil {
		s.metrics.IncWSQueueKick()
	}
	client.Close()
	return false
}

func (s *Server) writeEnvelopeConn(conn *websocket.Conn, env dto.Envelope) error {
	_ = conn.SetWriteDeadline(time.Now().Add(s.heartbeat.WriteWait))
	data, err := s.codec.EncodeEnvelope(env)
	if err != nil {
		return err
	}
	return conn.WriteMessage(s.codec.MessageType(), data)
}

func (s *Server) String() string {
	return fmt.Sprintf("ws-server(node=%s,addr=%s,max=%d)", s.NodeID, s.Addr, s.MaxConnections)
}

func (s *Server) SetDrainMode(enabled bool) {
	s.drainMode.Store(enabled)
}

func (s *Server) IsDrainMode() bool {
	return s.drainMode.Load()
}

// ConnectionCount 返回当前已预占的 WebSocket 连接槽位数量。
func (s *Server) ConnectionCount() int {
	return int(s.connections.Load())
}

func (s *Server) observeConnections() {
	if s.metrics != nil {
		s.metrics.SetWSConnections(s.connections.Load())
	}
}

func (s *Server) tryAcquireConnection() bool {
	for {
		current := s.connections.Load()
		if current >= int64(s.MaxConnections) {
			return false
		}
		if s.connections.CompareAndSwap(current, current+1) {
			return true
		}
	}
}
