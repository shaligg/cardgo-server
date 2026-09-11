package gameserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bigfish/go_orm_1/internal/domain/asset"
	inventorygame "github.com/bigfish/go_orm_1/internal/domain/inventory"
	playergame "github.com/bigfish/go_orm_1/internal/domain/player"
	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	battlegame "github.com/bigfish/go_orm_1/internal/gameplay/battle"
	cardgame "github.com/bigfish/go_orm_1/internal/gameplay/card"
	workshopgame "github.com/bigfish/go_orm_1/internal/gameplay/workshop"
	"github.com/bigfish/go_orm_1/internal/globalcore"
	"github.com/bigfish/go_orm_1/internal/handler"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	imetrics "github.com/bigfish/go_orm_1/internal/infra/metrics"
	iredis "github.com/bigfish/go_orm_1/internal/infra/redis"
	"github.com/bigfish/go_orm_1/internal/infra/websearch"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

// Application 持有 GameServer 的运行组件和基础设施资源，并统一管理其生命周期。
type Application struct {
	cfg                   Config
	apiServer             *http.Server
	wsServer              *ws.Server
	stateMaintainer       *state.Maintainer
	dbPool                io.Closer
	redisClient           io.Closer
	playerKickBus         *iredis.PlayerKickBus
	nodeRegistry          login.NodeRegistrar
	nodeInfo              login.NodeInfo
	nodeHeartbeatInterval time.Duration
	nodeTTL               time.Duration
	nodeHeartbeatCancel   context.CancelFunc
	nodeHeartbeatWG       sync.WaitGroup
}

func Bootstrap(ctx context.Context) (*Application, error) {
	_ = ctx
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if cfg.Auth.Algorithm != "hmac-sha256" {
		return nil, fmt.Errorf("unsupported auth algorithm: %s", cfg.Auth.Algorithm)
	}
	ticketSecret := os.Getenv(cfg.Auth.SecretEnvKey)
	if ticketSecret == "" {
		return nil, fmt.Errorf("auth ticket secret env %s is empty", cfg.Auth.SecretEnvKey)
	}
	adminToken := os.Getenv(cfg.Admin.TokenEnvKey)
	if cfg.Admin.RequireAuth && adminToken == "" {
		return nil, fmt.Errorf("admin token env %s is empty", cfg.Admin.TokenEnvKey)
	}
	dbDSN := os.Getenv(cfg.DB.DSNEnvKey)
	if dbDSN == "" {
		return nil, fmt.Errorf("db dsn env %s is empty", cfg.DB.DSNEnvKey)
	}
	metricsReg := imetrics.NewRegistry()
	redisClient, err := iredis.New(ctx, iredis.Config{
		Addr:     cfg.Redis.Addr,
		Password: os.Getenv(cfg.Redis.PasswordEnvKey),
		DB:       cfg.Redis.DB,
		Metrics:  metricsReg,
	})
	if err != nil {
		return nil, err
	}
	nodeRegistry := iredis.NewNodeRegistry(redisClient, cfg.Redis.NodeKeyPrefix)
	playerOwnerStore := iredis.NewPlayerOwnerStore(redisClient, cfg.Redis.PlayerOwnerKeyPrefix)
	playerKickBus := iredis.NewPlayerKickBus(redisClient, cfg.Redis.NodeKeyPrefix+":kick")

	gdb, err := idb.Open(idb.Config{
		DSN:                    dbDSN,
		MaxOpenConns:           cfg.DB.MaxOpenConns,
		MaxIdleConns:           cfg.DB.MaxIdleConns,
		ConnMaxLifetimeSeconds: cfg.DB.ConnMaxLifetimeSeconds,
		ConnMaxIdleTimeSeconds: cfg.DB.ConnMaxIdleTimeSeconds,
		Metrics:                metricsReg,
	})
	if err != nil {
		_ = redisClient.Close()
		return nil, err
	}
	dbPool, err := gdb.DB()
	if err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("get mysql connection pool: %w", err)
	}
	bootstrapComplete := false
	defer func() {
		if bootstrapComplete {
			return
		}
		if err := dbPool.Close(); err != nil {
			ilog.Errorf("close mysql after bootstrap failure: %v", err)
		}
		if err := redisClient.Close(); err != nil {
			ilog.Errorf("close redis after bootstrap failure: %v", err)
		}
	}()
	if err := repo.Migrate(gdb); err != nil {
		return nil, err
	}
	playerRepo := repo.NewDBPlayerRepository(gdb)
	assetRepo := repo.NewDBAssetRepository(gdb)
	cardRepo := repo.NewDBCardRepository(gdb)
	levelProgressRepo := &repo.DBLevelProgressRepository{}
	workshopRepo := repo.NewDBWorkshopRepository(gdb)
	friendRepo := repo.NewDBFriendRepository(gdb)
	guildRepo := repo.NewDBGuildRepository(gdb)
	chatRepo := repo.NewDBChatRepository(gdb)
	itemCatalog, err := gamedata.LoadItemCatalog(cfg.GameData.ItemConfigPath)
	if err != nil {
		return nil, err
	}
	gameData, err := gamedata.LoadGameData(gamedata.ConfigPaths{
		CardConfigPath:  cfg.GameData.CardConfigPath,
		OrderConfigPath: cfg.GameData.OrderConfigPath,
		LevelConfigPath: cfg.GameData.LevelConfigPath,
	}, itemCatalog)
	if err != nil {
		return nil, err
	}
	workshopData, err := gamedata.LoadWorkshopData(cfg.GameData.FacilityConfigPath, itemCatalog)
	if err != nil {
		return nil, err
	}
	assetService := asset.Service{Items: itemCatalog, PlayerRepo: assetRepo, InventoryRepo: assetRepo, Tx: idb.NewTxManager(gdb)}
	inventoryService := inventorygame.Service{Repo: assetRepo}
	playerService := playergame.Service{Repo: playerRepo, Assets: assetService}
	cardService := cardgame.Service{Repo: cardRepo, Assets: assetService, Tx: idb.NewTxManager(gdb), Data: gameData}
	battleService := &battlegame.Service{Data: gameData, Assets: assetService, Tx: idb.NewTxManager(gdb), Progress: levelProgressRepo}
	workshopService := workshopgame.Service{Repo: workshopRepo, Assets: assetService, Tx: idb.NewTxManager(gdb), Players: playerRepo, Data: workshopData}
	publicCore := globalcore.Core{
		Friend: globalcore.LocalFriendService{Repo: friendRepo, Players: playerRepo, Tx: idb.NewTxManager(gdb)},
		Guild:  globalcore.LocalGuildService{Repo: guildRepo, Messages: chatRepo, Tx: idb.NewTxManager(gdb)},
		Chat:   globalcore.LocalChatService{Messages: chatRepo, Membership: guildRepo},
	}
	shardExec := dispatcher.NewShardExecutor(cfg.Server.DispatcherShards)
	commandCache := session.NewCommandCache(time.Duration(cfg.State.OfflineTTLSec)*time.Second, 10, 16*1024)
	searchClient := websearch.NewClient(cfg.WebSearch.BaseURL, time.Duration(cfg.WebSearch.TimeoutMS)*time.Millisecond)

	nonceStore := auth.NewMemoryNonceStore()
	verifier := auth.Verifier{NonceStore: nonceStore, Secret: []byte(ticketSecret), Issuer: cfg.Auth.Issuer}
	sessionManager := session.NewMemoryManager()
	bizHandler := &handler.BizHandler{
		PlayerService:    playerService,
		AssetService:     assetService,
		InventoryService: inventoryService,
		CardService:      cardService,
		BattleService:    battleService,
		WorkshopService:  workshopService,
		FriendService:    publicCore.Friend,
		GuildService:     publicCore.Guild,
		ChatService:      publicCore.Chat,
		Searcher:         searchClient,
	}
	bizRouter := handler.NewRegisteredRouter(bizHandler, cfg.Debug.EnableWSDebugOps)
	bizDispatcher := handler.NewDispatcher(bizRouter, shardExec, commandCache)
	wsServer := ws.NewServer(ws.Options{
		NodeID:         cfg.Server.NodeID,
		Addr:           fmt.Sprintf("%s:%d", cfg.Server.WSHost, cfg.Server.WSPort),
		MaxConnections: cfg.Server.MaxConnections,
		DrainMode:      cfg.Server.DrainMode,
		Verifier:       verifier,
		SessionManager: sessionManager,
		Heartbeat: ws.HeartbeatConfig{
			Interval:  time.Duration(cfg.WS.HeartbeatIntervalSec) * time.Second,
			PongWait:  time.Duration(cfg.WS.PongWaitSec) * time.Second,
			WriteWait: time.Duration(cfg.WS.WriteWaitSec) * time.Second,
		},
		SendQueueSize:   cfg.WS.SendQueueSize,
		InboundMinGap:   time.Duration(cfg.WS.BizMinGapMS) * time.Millisecond,
		MaxMessageBytes: int64(cfg.WS.MaxMessageBytes),
		AllowedOrigins:  cfg.WS.AllowedOrigins,
		BizHandler:      bizDispatcher,
		OnSessionBound: func(ctx context.Context, uid string, connID string) error {
			previousOwner, err := playerOwnerStore.Claim(ctx, uid, cfg.Server.NodeID, connID, time.Duration(cfg.State.OwnerTTLSec)*time.Second)
			if err != nil {
				return err
			}
			// 无法证明归属连续时不复用本机局内状态和近期结果，避免 A -> B -> A 后恢复过期数据。
			if previousOwner.ServerID != cfg.Server.NodeID {
				battleService.DeletePlayerRuntime(uid)
				commandCache.Delete(uid)
			}
			if previousOwner.ServerID != "" && previousOwner.ServerID != cfg.Server.NodeID && previousOwner.ConnID != "" {
				notice := iredis.PlayerKickNotice{
					Target: iredis.PlayerKickTargetConnection,
					UID:    uid,
					ConnID: previousOwner.ConnID,
					Reason: "player connected to another game server",
				}
				if err := playerKickBus.Publish(ctx, previousOwner.ServerID, notice); err != nil {
					ilog.Errorf("publish player kick failed uid=%s old_server=%s old_conn=%s err=%v", uid, previousOwner.ServerID, previousOwner.ConnID, err)
				}
			}
			return nil
		},
		OnDisconnect: func(ctx context.Context, uid string, connID string) {
			// 旧连接被新连接替换后可能更晚触发断线回调，不能把新会话误标为离线。
			if current, ok, err := sessionManager.GetByUID(ctx, uid); err == nil && ok && current.ConnID != connID {
				return
			}
			if err := playerOwnerStore.MarkOffline(ctx, uid, cfg.Server.NodeID, connID, time.Duration(cfg.State.OfflineTTLSec)*time.Second); err != nil {
				ilog.Errorf("mark player owner offline failed uid=%s conn=%s err=%v", uid, connID, err)
			}
		},
		PreparePlayer: buildPreparePlayerCallback(playerService),
		Metrics:       metricsReg,
	})
	ownerReconciler := &playerOwnerReconciler{
		nodeID:   cfg.Server.NodeID,
		ownerTTL: time.Duration(cfg.State.OwnerTTLSec) * time.Second,
		owners:   playerOwnerStore,
		sessions: sessionManager,
		battles:  battleService,
		commands: commandCache,
		wsServer: wsServer,
	}
	stateMaintainer := state.NewMaintainer(state.MaintainerOptions{
		OwnerCheckInterval: time.Duration(cfg.State.OwnerCheckIntervalSec) * time.Second,
		OwnerReconciler:    ownerReconciler,
	})

	loginService := login.Service{
		Allocator: login.RegistryNodeAllocator{
			Registry:   nodeRegistry,
			LastServer: playerOwnerStore,
		},
		Issuer: login.LocalTicketIssuer{
			TTL:    time.Duration(cfg.Auth.TicketTTLSec) * time.Second,
			Secret: []byte(ticketSecret),
			Issuer: cfg.Auth.Issuer,
		},
	}

	apiAddr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	apiServer := &http.Server{
		Addr:    apiAddr,
		Handler: buildAPIMux(cfg, adminToken, wsServer, metricsReg, sessionManager, loginService),
	}

	app := &Application{
		cfg:             cfg,
		apiServer:       apiServer,
		wsServer:        wsServer,
		stateMaintainer: stateMaintainer,
		dbPool:          dbPool,
		redisClient:     redisClient,
		playerKickBus:   playerKickBus,
		nodeRegistry:    nodeRegistry,
		nodeInfo: login.NodeInfo{
			ServerID:  cfg.Server.NodeID,
			WSAddr:    cfg.Server.AdvertisedWSAddr,
			MaxOnline: cfg.Server.MaxConnections,
			Healthy:   true,
			Drain:     cfg.Server.DrainMode,
		},
		nodeHeartbeatInterval: time.Duration(cfg.Redis.NodeHeartbeatSec) * time.Second,
		nodeTTL:               time.Duration(cfg.Redis.NodeTTLSec) * time.Second,
	}
	bootstrapComplete = true
	ilog.Infof("bootstrap done node=%s api=%s ws=%s", cfg.Server.NodeID, apiAddr, wsServer.Addr)
	return app, nil
}
