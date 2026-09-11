package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/bigfish/go_orm_1/internal/app/loginserver"
)

// main 只管理独立登录进程，不启动游戏服或注册游戏节点。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	app, err := loginserver.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	if err := app.Start(ctx); err != nil {
		log.Fatalf("start failed: %v", err)
	}
	<-ctx.Done()
	if err := app.Stop(context.Background()); err != nil {
		log.Printf("stop with error: %v", err)
	}
}
