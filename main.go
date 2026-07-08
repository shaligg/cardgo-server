package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/bigfish/go_orm_1/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.Bootstrap(ctx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	if err := a.Start(ctx); err != nil {
		log.Fatalf("start failed: %v", err)
	}

	<-ctx.Done()
	if err := a.Stop(context.Background()); err != nil {
		log.Printf("stop with error: %v", err)
	}
}
