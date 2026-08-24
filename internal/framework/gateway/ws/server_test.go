package ws

import (
	"context"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOriginChecker(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		origin  string
		want    bool
	}{
		{name: "native client without origin", want: true},
		{name: "browser rejected without allowlist", origin: "https://game.example.com", want: false},
		{name: "configured browser origin", allowed: []string{"https://game.example.com"}, origin: "https://game.example.com", want: true},
		{name: "other browser origin", allowed: []string{"https://game.example.com"}, origin: "https://evil.example.com", want: false},
		{name: "local wildcard", allowed: []string{"*"}, origin: "http://127.0.0.1:3000", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://game.example.com/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			server := NewServer(Options{AllowedOrigins: tt.allowed})
			if got := server.upgrader.CheckOrigin(req); got != tt.want {
				t.Fatalf("allowed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerStartReturnsListenError(t *testing.T) {
	server := NewServer(Options{Addr: "127.0.0.1:not-a-port"})
	if err := server.Start(context.Background()); err == nil {
		t.Fatal("Start should return listener error")
	}
}

func TestTryAcquireConnectionNeverExceedsLimit(t *testing.T) {
	server := NewServer(Options{MaxConnections: 8})
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if server.tryAcquireConnection() {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()

	if accepted.Load() != 8 || server.connections.Load() != 8 {
		t.Fatalf("accepted=%d connections=%d, want 8", accepted.Load(), server.connections.Load())
	}
}

func TestServerStopWaitsForActiveHandler(t *testing.T) {
	server := NewServer(Options{})
	if !server.beginHandler() {
		t.Fatal("beginHandler should accept before stop")
	}
	handlerDone := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(handlerDone)
		server.handlerWG.Done()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	select {
	case <-handlerDone:
	default:
		t.Fatal("Stop returned before active handler completed")
	}
}
