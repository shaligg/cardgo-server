package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	Codec           EnvelopeCodec
	BizHandler      BizHandler
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

type messagePriority int

const (
	priorityHigh messagePriority = iota
	priorityNormal
	priorityLow
)

type Server struct {
	Addr           string
	NodeID         string
	MaxConnections int

	verifier       auth.TicketVerifier
	sessionManager session.Manager
	heartbeat      HeartbeatConfig
	bizHandler     BizHandler
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
		onDisconnect:    opts.OnDisconnect,
		onRestoreState:  opts.OnRestoreState,
		metrics:         opts.Metrics,
		codec:           opts.Codec,
		sendQueueSize:   opts.SendQueueSize,
		inboundLimiter:  limiter,
		maxMessageBytes: opts.MaxMessageBytes,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients: make(map[string]*Client),
	}
	s.drainMode.Store(opts.DrainMode)
	return s
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

	go func() {
		ilog.Infof("ws server listening on %s", s.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("ws server stopped with error: %v", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	for _, c := range s.clients {
		c.Close()
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if s.IsDrainMode() || s.connections.Load() >= int64(s.MaxConnections) {
		s.writeServerFullHTTP(w)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.connections.Add(1)
	s.observeConnections()
	defer func() {
		s.connections.Add(-1)
		s.observeConnections()
		_ = conn.Close()
	}()

	conn.SetReadLimit(s.maxMessageBytes)
	_ = conn.SetReadDeadline(time.Now().Add(s.heartbeat.PongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(s.heartbeat.PongWait))
	})

	uid, connID, client, ok := s.authenticate(r.Context(), conn)
	if !ok {
		return
	}
	defer func() {
		_ = s.sessionManager.Unbind(context.Background(), uid, connID)
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
			resp, bizErr := s.bizHandler.Handle(r.Context(), uid, req.OpCode, req.Payload)
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

	cnt, err := s.sessionManager.Count(ctx)
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, first.Seq, terrors.CodeInternal, "session count failed")
		return "", "", nil, false
	}
	if s.IsDrainMode() || cnt >= s.MaxConnections {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeServerFullWS(conn, first.Seq)
		return "", "", nil, false
	}

	connID = uuid.NewString()
	oldConnID, err := s.sessionManager.Bind(ctx, session.Session{
		UID:      claims.UID,
		ConnID:   connID,
		LoginAt:  time.Now(),
		LastSeen: time.Now(),
		ClientIP: "",
	})
	if err != nil {
		if s.metrics != nil {
			s.metrics.IncWSAuthFailed()
		}
		_ = s.writeErrorConn(conn, first.Seq, terrors.CodeInternal, "session bind failed")
		return "", "", nil, false
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

	switch priorityByType(env.Type) {
	case priorityLow:
		ilog.Infof("drop low-priority message uid=%s conn=%s type=%s", client.UID, client.ConnID, env.Type)
		if s.metrics != nil {
			s.metrics.IncWSQueueDrop()
		}
		return true
	default:
		ilog.Errorf("send queue full uid=%s conn=%s type=%s", client.UID, client.ConnID, env.Type)
		if s.metrics != nil {
			s.metrics.IncWSQueueKick()
		}
		client.Close()
		return false
	}
}

func priorityByType(msgType string) messagePriority {
	switch msgType {
	case dto.TypePush:
		return priorityLow
	case dto.TypeBizAck:
		return priorityNormal
	default:
		return priorityHigh
	}
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

func (s *Server) observeConnections() {
	if s.metrics != nil {
		s.metrics.SetWSConnections(s.connections.Load())
	}
}
