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

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	iredis "github.com/bigfish/go_orm_1/internal/infra/redis"
	"github.com/bigfish/go_orm_1/internal/platform/account"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/go-sql-driver/mysql"
)

// Application 独立持有账号 MySQL、HTTP 和 Redis；不管理 GameServer 长连接。
type Application struct {
	cfg         Config
	httpServer  *http.Server
	redisClient io.Closer
	dbClient    io.Closer
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

	dsn, err := mysql.ParseDSN(os.Getenv(cfg.DB.DSNEnvKey))
	if err != nil || os.Getenv(cfg.DB.DSNEnvKey) == "" {
		return nil, fmt.Errorf("account database DSN missing or invalid")
	}
	dsn.Timeout, dsn.ReadTimeout, dsn.WriteTimeout = 5*time.Second, 5*time.Second, 5*time.Second
	gdb, err := idb.Open(idb.Config{DSN: dsn.FormatDSN(), MaxOpenConns: cfg.DB.MaxOpenConns, MaxIdleConns: cfg.DB.MaxIdleConns, ConnMaxLifetimeSeconds: cfg.DB.ConnMaxLifetimeSec, ConnMaxIdleTimeSeconds: cfg.DB.ConnMaxIdleTimeSec, RedactParameters: true})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	keepDB := false
	defer func() {
		if !keepDB {
			_ = sqlDB.Close()
		}
	}()
	repository := repo.NewDBAccountRepository(gdb)
	if err := repository.CheckSchema(ctx); err != nil {
		return nil, fmt.Errorf("account schema unavailable: %w", err)
	}
	accounts, err := account.NewService(repository, idb.NewTxManager(gdb), time.Duration(cfg.Account.SessionIdleTTLSec)*time.Second)
	if err != nil {
		return nil, err
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
	keepDB = true
	return &Application{
		cfg:         cfg,
		redisClient: client,
		dbClient:    sqlDB,
		httpServer: &http.Server{
			Addr:              net.JoinHostPort(cfg.HTTP.Host, strconv.Itoa(cfg.HTTP.Port)),
			Handler:           buildHTTPMux(accounts, service, cfg.HTTPSecurity),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      15 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    16384,
		},
	}, nil
}
