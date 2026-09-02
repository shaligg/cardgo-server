package redis

import (
	"context"
	"net"
	"time"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	goredis "github.com/redis/go-redis/v9"
)

// metricsHook 在 go-redis 的统一执行入口采集命令和 Pipeline 耗时。
type metricsHook struct {
	registry *metrics.Registry
}

func (h metricsHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h metricsHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		startedAt := time.Now()
		err := next(ctx, cmd)
		h.registry.ObserveRedisDuration(time.Since(startedAt))
		return err
	}
}

func (h metricsHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		startedAt := time.Now()
		err := next(ctx, cmds)
		h.registry.ObserveRedisDuration(time.Since(startedAt))
		return err
	}
}
