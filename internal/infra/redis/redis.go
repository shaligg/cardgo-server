package redis

import (
	"context"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string
	Password string
	DB       int
	Metrics  *metrics.Registry
}

// Client 包装项目共享的 Redis 连接。
type Client struct {
	raw *goredis.Client
}

// New 创建 Redis 客户端并验证连接可用。
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis addr is empty")
	}
	raw := goredis.NewClient(&goredis.Options{Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB})
	if cfg.Metrics != nil {
		raw.AddHook(metricsHook{registry: cfg.Metrics})
	}
	if err := raw.Ping(ctx).Err(); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("ping redis %s: %w", cfg.Addr, err)
	}
	return &Client{raw: raw}, nil
}

// Close 关闭 Redis 连接。
func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}
