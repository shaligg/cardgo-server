package gameserver

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/platform/login"
)

type recordingNodeRegistrar struct {
	node login.NodeInfo
	ttl  time.Duration
}

type recoveringNodeRegistrar struct {
	mu        sync.Mutex
	calls     int
	recovered chan struct{}
	once      sync.Once
}

type recordingCloser struct {
	calls int
	err   error
}

func (c *recordingCloser) Close() error {
	c.calls++
	return c.err
}

func (r *recoveringNodeRegistrar) UpsertNode(context.Context, login.NodeInfo, time.Duration) error {
	r.mu.Lock()
	r.calls++
	calls := r.calls
	r.mu.Unlock()
	if calls == 1 {
		return errors.New("redis unavailable")
	}
	r.once.Do(func() { close(r.recovered) })
	return nil
}

func (r *recoveringNodeRegistrar) RemoveNode(context.Context, string) error {
	return nil
}

func (r *recoveringNodeRegistrar) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *recordingNodeRegistrar) UpsertNode(ctx context.Context, node login.NodeInfo, ttl time.Duration) error {
	_ = ctx
	r.node = node
	r.ttl = ttl
	return nil
}

func (r *recordingNodeRegistrar) RemoveNode(ctx context.Context, serverID string) error {
	_ = ctx
	_ = serverID
	return nil
}

func TestApplicationStartReturnsAPIListenError(t *testing.T) {
	dbPool := &recordingCloser{}
	redisClient := &recordingCloser{}
	app := &Application{
		apiServer:   &http.Server{Addr: "127.0.0.1:not-a-port"},
		dbPool:      dbPool,
		redisClient: redisClient,
	}
	if err := app.Start(context.Background()); err == nil {
		t.Fatal("Start should return API listener error")
	}
	if dbPool.calls != 1 || redisClient.calls != 1 {
		t.Fatalf("close calls db=%d redis=%d, want 1 each", dbPool.calls, redisClient.calls)
	}
}

func TestCloseInfrastructureContinuesAfterCloseError(t *testing.T) {
	dbErr := errors.New("close mysql failed")
	dbPool := &recordingCloser{err: dbErr}
	redisClient := &recordingCloser{}
	app := &Application{dbPool: dbPool, redisClient: redisClient}

	if err := app.closeInfrastructure(); !errors.Is(err, dbErr) {
		t.Fatalf("closeInfrastructure error = %v, want %v", err, dbErr)
	}
	if dbPool.calls != 1 || redisClient.calls != 1 {
		t.Fatalf("close calls db=%d redis=%d, want 1 each", dbPool.calls, redisClient.calls)
	}
	if err := app.closeInfrastructure(); err != nil {
		t.Fatalf("second closeInfrastructure returned error: %v", err)
	}
	if dbPool.calls != 1 || redisClient.calls != 1 {
		t.Fatalf("second close repeated calls db=%d redis=%d", dbPool.calls, redisClient.calls)
	}
}

func TestApplicationReportNodeUsesCurrentWSState(t *testing.T) {
	registry := &recordingNodeRegistrar{}
	app := &Application{
		wsServer:     ws.NewServer(ws.Options{DrainMode: true}),
		nodeRegistry: registry,
		nodeInfo: login.NodeInfo{
			ServerID:  "gs-a",
			WSAddr:    "ws://gs-a/ws",
			MaxOnline: 2000,
			Healthy:   true,
		},
		nodeTTL: 15 * time.Second,
	}

	if err := app.reportNode(context.Background()); err != nil {
		t.Fatalf("reportNode returned error: %v", err)
	}
	if registry.node.ServerID != "gs-a" || registry.node.Online != 0 || !registry.node.Drain {
		t.Fatalf("unexpected reported node: %+v", registry.node)
	}
	if registry.ttl != 15*time.Second {
		t.Fatalf("expected ttl 15s, got %s", registry.ttl)
	}
}

func TestNodeHeartbeatRetriesAfterRegistryFailure(t *testing.T) {
	registry := &recoveringNodeRegistrar{recovered: make(chan struct{})}
	app := &Application{
		wsServer:              ws.NewServer(ws.Options{}),
		nodeRegistry:          registry,
		nodeInfo:              login.NodeInfo{ServerID: "gs-a", WSAddr: "ws://gs-a/ws", Healthy: true},
		nodeTTL:               time.Second,
		nodeHeartbeatInterval: 5 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.startNodeHeartbeat(ctx)

	select {
	case <-registry.recovered:
	case <-time.After(time.Second):
		t.Fatal("node heartbeat did not retry successfully after registry recovery")
	}
	cancel()
	app.nodeHeartbeatWG.Wait()
	if calls := registry.callCount(); calls < 2 {
		t.Fatalf("upsert calls = %d, want at least 2", calls)
	}
}
