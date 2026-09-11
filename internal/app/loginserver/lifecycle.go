package loginserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
)

// Start 同步绑定端口，监听失败时向入口返回错误并释放 Redis。
func (a *Application) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(err, a.closeRedis())
	}
	listener, err := net.Listen("tcp", a.httpServer.Addr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen login http %s: %w", a.httpServer.Addr, err), a.closeRedis())
	}
	go func() {
		ilog.Infof("login server listening on %s", listener.Addr())
		if err := a.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("login server stopped with error: %v", err)
		}
	}()
	return nil
}

// Stop 先等 HTTP 请求结束；超过停服期限则关闭连接，最后释放 Redis。
func (a *Application) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := a.httpServer.Shutdown(shutdownCtx)
	if err != nil {
		err = errors.Join(err, a.httpServer.Close())
	}
	return errors.Join(err, a.closeRedis())
}

func (a *Application) closeRedis() error {
	if a.redisClient == nil {
		return nil
	}
	client := a.redisClient
	a.redisClient = nil
	return client.Close()
}
