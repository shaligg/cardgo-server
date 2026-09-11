package loginserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	iredis "github.com/bigfish/go_orm_1/internal/infra/redis"
	"github.com/bigfish/go_orm_1/internal/platform/login"
)

// Application 只持有登录 HTTP 与 Redis，关闭登录进程不会影响 GameServer 的长连接。
type Application struct {
	cfg         Config
	httpServer  *http.Server
	redisClient io.Closer
}

// Bootstrap 校验配置后复用共享节点表、玩家归属读取器和票据签发器。
func Bootstrap(ctx context.Context) (*Application, error) {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	secret := os.Getenv(cfg.Auth.SecretEnvKey)
	if secret == "" {
		return nil, fmt.Errorf("auth ticket secret env %s is empty", cfg.Auth.SecretEnvKey)
	}
	// Redis 构造失败时由 New 释放连接；成功后所有资源由 Application 管理。
	client, err := iredis.New(ctx, iredis.Config{
		Addr: cfg.Redis.Addr, Password: os.Getenv(cfg.Redis.PasswordEnvKey), DB: cfg.Redis.DB,
	})
	if err != nil {
		return nil, err
	}
	service := login.Service{
		Allocator: login.RegistryNodeAllocator{
			Registry:   iredis.NewNodeRegistry(client, cfg.Redis.NodeKeyPrefix),
			LastServer: iredis.NewPlayerOwnerStore(client, cfg.Redis.PlayerOwnerKeyPrefix),
		},
		Issuer: login.LocalTicketIssuer{
			TTL: time.Duration(cfg.Auth.TicketTTLSec) * time.Second, Secret: []byte(secret), Issuer: cfg.Auth.Issuer,
		},
		// 不设置 LastServer 写入器：玩家归属只能由 GameServer 验票并绑定成功后认领。
	}
	return &Application{
		cfg:         cfg,
		redisClient: client,
		httpServer: &http.Server{
			Addr:              net.JoinHostPort(cfg.HTTP.Host, strconv.Itoa(cfg.HTTP.Port)),
			Handler:           buildHTTPMux(service),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

func buildHTTPMux(service login.Provider) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/login", login.NewHTTPHandler(service))
	// 这里只报告 HTTP 进程存活；节点可用性仍由实际登录分配结果表达。
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"ok\":true}\n")
	})
	return mux
}
