package gameserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/eventbus"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/platform/session"
	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

type Application struct {
	cfg         Config
	bus         eventbus.Bus
	loginSvc    login.Provider
	apiServer   *http.Server
	wsServer    *ws.Server
	flushQueue  state.FlushQueue
	flushWorker *state.FlushWorker
	onlineState *state.OnlineState
	metricsReg  *imetrics.Registry
}

func Bootstrap(ctx context.Context) (*Application, error) {
	_ = ctx
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return nil, err
	}

	gdb, err := idb.Open(idb.Config{DSN: cfg.DB.DSN})
	if err != nil {
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
	flushWorker := state.NewFlushWorker(flushQueue, onlineState, snapshotRepo, state.FlushWorkerOptions{
		BatchSize: 128,
		Interval:  200 * time.Millisecond,
		MaxRetry:  cfg.Server.FlushMaxRetry,
		Observer:  newFlushMetricsObserver(metricsReg),
	})
	shardExec := dispatcher.NewShardExecutor(cfg.Server.DispatcherShards)

	nonceStore := auth.NewMemoryNonceStore()
	verifier := auth.Verifier{NonceStore: nonceStore}
	sessionManager := session.NewMemoryManager()
	lastServerStore := session.NewMemoryLastServerStore()
	bizRouter := handler.NewRegisteredRouter(playerService, assetService, inventoryService, cardService, battleService, workshopService, onlineState, cfg.Debug.EnableWSDebugOps)
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
		BizHandler:      bizDispatcher,
		OnDisconnect: func(ctx context.Context, uid string, connID string) {
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

	wsHostForClient := cfg.Server.WSHost
	if wsHostForClient == "" || wsHostForClient == "0.0.0.0" {
		wsHostForClient = "127.0.0.1"
	}
	loginService := login.Service{
		Allocator: login.RegistryNodeAllocator{
			Registry: login.StaticNodeRegistry{
				Nodes: []login.NodeInfo{{
					ServerID:  cfg.Server.NodeID,
					WSAddr:    fmt.Sprintf("ws://%s:%d/ws", wsHostForClient, cfg.Server.WSPort),
					MaxOnline: cfg.Server.MaxConnections,
					Healthy:   true,
					Drain:     cfg.Server.DrainMode,
				}},
			},
			LastServer: lastServerStore,
		},
		LastServer: lastServerStore,
		Issuer: login.LocalTicketIssuer{
			TTL: time.Duration(cfg.Auth.TicketTTLSec) * time.Second,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		drainMode := wsServer.IsDrainMode()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ready":      !drainMode,
			"drain_mode": drainMode,
			"node_id":    cfg.Server.NodeID,
		})
	})
	mux.HandleFunc("/metricsz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, metricsReg.Snapshot())
	})
	mux.HandleFunc("/admin/drain", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"code": 0,
				"msg":  "ok",
				"data": map[string]interface{}{
					"drain_mode": wsServer.IsDrainMode(),
				},
			})
		case http.MethodPost:
			var req struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{
					"code": "BAD_REQUEST",
					"msg":  "invalid json",
				})
				return
			}
			wsServer.SetDrainMode(req.Enabled)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"code": 0,
				"msg":  "ok",
				"data": map[string]interface{}{
					"drain_mode": wsServer.IsDrainMode(),
				},
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/admin/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		count, err := sessionManager.Count(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"code": "INTERNAL_ERROR",
				"msg":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"active_sessions": count,
				"drain_mode":      wsServer.IsDrainMode(),
			},
		})
	})
	mux.Handle("/api/login", login.NewHTTPHandler(loginService))

	apiAddr := fmt.Sprintf("%s:%d", cfg.Server.APIHost, cfg.Server.APIPort)
	apiServer := &http.Server{
		Addr:    apiAddr,
		Handler: mux,
	}

	ilog.Infof("bootstrap done node=%s api=%s ws=%s", cfg.Server.NodeID, apiAddr, wsServer.Addr)
	return &Application{
		cfg:         cfg,
		bus:         eventbus.NewInProcBus(),
		loginSvc:    loginService,
		apiServer:   apiServer,
		wsServer:    wsServer,
		flushQueue:  flushQueue,
		flushWorker: flushWorker,
		onlineState: onlineState,
		metricsReg:  metricsReg,
	}, nil
}
