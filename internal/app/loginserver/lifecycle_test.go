package loginserver

import (
	"context"
	"net"
	"net/http"
	"testing"
)

type recordingCloser struct{ calls int }

func (c *recordingCloser) Close() error { c.calls++; return nil }

func TestStartReleasesRedisWhenPortOccupied(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client := &recordingCloser{}
	database := &recordingCloser{}
	app := &Application{httpServer: &http.Server{Addr: listener.Addr().String()}, redisClient: client, dbClient: database}
	if err := app.Start(context.Background()); err == nil {
		t.Fatal("occupied port should fail")
	}
	if client.calls != 1 || database.calls != 1 {
		t.Fatalf("redis close calls = %d", client.calls)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || database.calls != 1 {
		t.Fatal("Stop closed Redis twice")
	}
}

func TestStopClosesHTTPAndRedis(t *testing.T) {
	client := &recordingCloser{}
	database := &recordingCloser{}
	server := &http.Server{Addr: "127.0.0.1:0"}
	app := &Application{httpServer: server, redisClient: client, dbClient: database}
	if err := app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := app.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Shutdown 后再次 Serve 必须返回 ErrServerClosed，Redis 也只能关闭一次。
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := server.Serve(listener); err != http.ErrServerClosed {
		t.Fatalf("Serve after Stop = %v", err)
	}
	if client.calls != 1 || database.calls != 1 {
		t.Fatalf("redis close calls = %d", client.calls)
	}
}
