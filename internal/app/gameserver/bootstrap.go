package gameserver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/game/asset"
	battlegame "github.com/bigfish/go_orm_1/internal/game/battle"
	cardgame "github.com/bigfish/go_orm_1/internal/game/card"
	inventorygame "github.com/bigfish/go_orm_1/internal/game/inventory"
	playergame "github.com/bigfish/go_orm_1/internal/game/player"
	workshopgame "github.com/bigfish/go_orm_1/internal/game/workshop"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	"github.com/bigfish/go_orm_1/internal/handler"
	"github.com/bigfish/go_orm_1/internal/infra/cache"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	imetrics "github.com/bigfish/go_orm_1/internal/infra/metrics"
	iredis "github.com/bigfish/go_orm_1/internal/infra/redis"
	"github.com/bigfish/go_orm_1/internal/infra/websearch"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/eventbus"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

type Application struct {
	cfg                   Config
	bus                   eventbus.Bus
	loginSvc              login.Provider
	apiServer             *http.Server
	wsServer              *ws.Server
	flushQueue            state.FlushQueue
	flushWorker           *state.FlushWorker
	onlineState           *state.OnlineState
	metricsReg            *imetrics.Registry
	redisClient           *iredis.Client
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
	redisClient, err := iredis.New(ctx, iredis.Config{
		Addr:     cfg.Redis.Addr,
		Password: os.Getenv(cfg.Redis.PasswordEnvKey),
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return nil, err
	}
	nodeRegistry := iredis.NewNodeRegistry(redisClient, cfg.Redis.NodeKeyPrefix)
	playerOwnerStore := iredis.NewPlayerOwnerStore(redisClient, cfg.Redis.PlayerOwnerKeyPrefix)

	gdb, err := idb.Open(idb.Config{DSN: cfg.DB.DSN})
	if err != nil {
		_ = redisClient.Close()
		return nil, err
	}
	dbRepo := repo.NewDBPlayerRepository(gdb)
	if err := dbRepo.Migrate(); err != nil {
		return nil, err
	}
	snapshotRepo := repo.NewDBSnapshotRepository(gdb)
	if err := snapshotRepo.Migrate(); err != nil {
		return nil, err
	}
	cachedRepo := repo.NewCachedPlayerRepository(
		dbRepo,
		cache.NewL1Cache(),
		time.Duration(cfg.Cache.L1TTLSec)*time.Second,
	)
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
	assetService := asset.Service{Items: itemCatalog, Players: cachedRepo, Inventory: dbRepo, Tx: idb.NewTxManager(gdb), TxPlayers: dbRepo, TxInventory: dbRepo}
	inventoryService := inventorygame.Service{Repo: dbRepo}
	playerService := playergame.Service{Repo: cachedRepo, Assets: assetService}
	cardService := cardgame.Service{Repo: dbRepo, Assets: assetService, Tx: idb.NewTxManager(gdb), Data: gameData, PlayerCache: cachedRepo}
	battleService := &battlegame.Service{Data: gameData, Assets: assetService, Tx: idb.NewTxManager(gdb)}
	workshopService := workshopgame.Service{Repo: dbRepo, Assets: assetService, Tx: idb.NewTxManager(gdb), PlayerCache: cachedRepo, Players: cachedRepo, Data: workshopData}
	onlineState := state.NewOnlineState()
	flushQueue := state.NewMemoryFlushQueue(cfg.Server.FlushQueueMax)
	metricsReg := imetrics.NewRegistry()
	shardExec := dispatcher.NewShardExecutor(cfg.Server.DispatcherShards)
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
		Searcher:         searchClient,
		Online:           onlineState,
	}
	bizRouter := handler.NewRegisteredRouter(bizHandler, cfg.Debug.EnableWSDebugOps)
	bizDispatcher := handler.NewDispatcher(bizRouter, shardExec)
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
			previousServerID, err := playerOwnerStore.Claim(ctx, uid, cfg.Server.NodeID, connID, time.Duration(cfg.State.OwnerTTLSec)*time.Second)
			if err != nil {
				return err
			}
			// 无法证明归属连续时不复用本机旧状态，避免 A -> B -> A 后恢复 A 的过期副本。
			if previousServerID != cfg.Server.NodeID {
				onlineState.Delete(uid)
				battleService.DeletePlayerRuntime(uid)
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
			onlineState.MarkOffline(uid, time.Duration(cfg.State.OfflineTTLSec)*time.Second)
			if err := flushQueue.Enqueue(ctx, state.FlushTask{UID: uid}); err != nil {
				ilog.Errorf("enqueue flush failed uid=%s conn=%s err=%v", uid, connID, err)
				return
			}
			metricsReg.IncFlushEnqueued()
			metricsReg.SetFlushQueueLen(int64(flushQueue.Len()))
		},
		OnRestoreState: buildRestoreStateCallback(onlineState, snapshotRepo),
		Metrics:        metricsReg,
	})
	ownerReconciler := &playerOwnerReconciler{
		nodeID:   cfg.Server.NodeID,
		ownerTTL: time.Duration(cfg.State.OwnerTTLSec) * time.Second,
		owners:   playerOwnerStore,
		sessions: sessionManager,
		online:   onlineState,
		battles:  battleService,
		wsServer: wsServer,
	}
	flushWorker := state.NewFlushWorker(flushQueue, onlineState, snapshotRepo, state.FlushWorkerOptions{
		BatchSize:          128,
		Interval:           200 * time.Millisecond,
		MaxRetry:           cfg.Server.FlushMaxRetry,
		CleanupInterval:    time.Duration(cfg.State.CleanupIntervalSec) * time.Second,
		OwnerCheckInterval: time.Duration(cfg.State.OwnerCheckIntervalSec) * time.Second,
		OwnerReconciler:    ownerReconciler,
		Observer:           newFlushMetricsObserver(metricsReg),
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

	ilog.Infof("bootstrap done node=%s api=%s ws=%s", cfg.Server.NodeID, apiAddr, wsServer.Addr)
	return &Application{
		cfg:          cfg,
		bus:          eventbus.NewInProcBus(),
		loginSvc:     loginService,
		apiServer:    apiServer,
		wsServer:     wsServer,
		flushQueue:   flushQueue,
		flushWorker:  flushWorker,
		onlineState:  onlineState,
		metricsReg:   metricsReg,
		redisClient:  redisClient,
		nodeRegistry: nodeRegistry,
		nodeInfo: login.NodeInfo{
			ServerID:  cfg.Server.NodeID,
			WSAddr:    cfg.Server.AdvertisedWSAddr,
			MaxOnline: cfg.Server.MaxConnections,
			Healthy:   true,
			Drain:     cfg.Server.DrainMode,
		},
		nodeHeartbeatInterval: time.Duration(cfg.Redis.NodeHeartbeatSec) * time.Second,
		nodeTTL:               time.Duration(cfg.Redis.NodeTTLSec) * time.Second,
	}, nil
}
