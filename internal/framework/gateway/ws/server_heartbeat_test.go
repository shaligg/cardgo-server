package ws

import (
	"context"
	"fmt"
	"testing"
	"time"

	dto "github.com/bigfish/go_orm_1/internal/framework/transport/dto"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/gorilla/websocket"
)

func TestValidMessagesKeepConnectionAliveBeyondTwoTimeouts(t *testing.T) {
	const pongWait = 250 * time.Millisecond
	tests := []struct {
		name         string
		requestType  string
		responseType string
		opCode       int32
	}{
		{name: "heartbeat", requestType: dto.TypeHeartbeatReq, responseType: dto.TypeHeartbeatAck},
		{name: "business", requestType: dto.TypeBizReq, responseType: dto.TypeBizAck, opCode: 1001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startHeartbeatTestServer(t, pongWait)
			conn := dialAuthenticatedTestClient(t, server)
			defer conn.Close()

			deadline := time.Now().Add(2*pongWait + pongWait/2)
			var seq int64
			for time.Now().Before(deadline) {
				seq++
				if err := conn.WriteJSON(dto.Envelope{
					Seq:     seq,
					Type:    tt.requestType,
					OpCode:  tt.opCode,
					TS:      time.Now().Unix(),
					Payload: map[string]interface{}{},
				}); err != nil {
					t.Fatalf("write %s after %s: %v", tt.requestType, time.Until(deadline), err)
				}

				_ = conn.SetReadDeadline(time.Now().Add(pongWait))
				var response dto.RawEnvelope
				if err := conn.ReadJSON(&response); err != nil {
					t.Fatalf("read %s response: %v", tt.requestType, err)
				}
				if response.Type != tt.responseType {
					t.Fatalf("response type = %s, want %s", response.Type, tt.responseType)
				}
				time.Sleep(pongWait / 5)
			}
		})
	}
}

func TestInvalidEnvelopeDoesNotRefreshReadDeadline(t *testing.T) {
	const pongWait = 250 * time.Millisecond
	server := startHeartbeatTestServer(t, pongWait)
	conn := dialAuthenticatedTestClient(t, server)
	defer conn.Close()

	startedAt := time.Now()
	deadline := time.Now().Add(3 * pongWait)
	for time.Now().Before(deadline) {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{`)); err != nil {
			if time.Since(startedAt) < pongWait/2 {
				t.Fatalf("connection closed before read deadline: %v", err)
			}
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		if _, _, err := conn.ReadMessage(); err != nil {
			if time.Since(startedAt) < pongWait/2 {
				t.Fatalf("connection closed before read deadline: %v", err)
			}
			return
		}
		time.Sleep(pongWait / 5)
	}
	t.Fatal("invalid envelopes kept the connection alive beyond the original read deadline")
}

func startHeartbeatTestServer(t *testing.T, pongWait time.Duration) *Server {
	t.Helper()
	server := NewServer(Options{
		NodeID:         "heartbeat-test-node",
		Addr:           "127.0.0.1:0",
		MaxConnections: 10,
		Verifier: auth.Verifier{
			Secret:     []byte("heartbeat-test-secret"),
			Issuer:     "heartbeat-test",
			NonceStore: auth.NewMemoryNonceStore(),
		},
		SessionManager: session.NewMemoryManager(),
		Heartbeat: HeartbeatConfig{
			PongWait:  pongWait,
			WriteWait: pongWait,
		},
	})
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("start test server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Stop(ctx); err != nil {
			t.Errorf("stop test server: %v", err)
		}
	})
	return server
}

func dialAuthenticatedTestClient(t *testing.T, server *Server) *websocket.Conn {
	t.Helper()
	ticket, err := auth.SignTicket(auth.TicketClaims{
		UID:      "heartbeat-user",
		ServerID: server.NodeID,
		ExpUnix:  time.Now().Add(time.Minute).Unix(),
		Nonce:    fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()),
		Issuer:   "heartbeat-test",
	}, []byte("heartbeat-test-secret"))
	if err != nil {
		t.Fatalf("sign test ticket: %v", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial("ws://"+server.Addr+"/ws", nil)
	if err != nil {
		t.Fatalf("dial test server: %v", err)
	}
	if err := conn.WriteJSON(dto.Envelope{
		Seq:  1,
		Type: dto.TypeAuthReq,
		TS:   time.Now().Unix(),
		Payload: dto.AuthReqPayload{
			Ticket: ticket,
		},
	}); err != nil {
		_ = conn.Close()
		t.Fatalf("write auth request: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var response dto.RawEnvelope
	if err := conn.ReadJSON(&response); err != nil {
		_ = conn.Close()
		t.Fatalf("read auth response: %v", err)
	}
	if response.Type != dto.TypeAuthAck {
		_ = conn.Close()
		t.Fatalf("auth response type = %s, want %s", response.Type, dto.TypeAuthAck)
	}
	return conn
}
