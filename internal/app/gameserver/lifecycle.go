package gameserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
	iredis "github.com/bigfish/go_orm_1/internal/infra/redis"
)

// Start 按 API 占位、顶号订阅、WS 监听、节点注册的顺序启动应用，避免注册尚未就绪的节点。
func (a *Application) Start(ctx context.Context) error {
	adminListener, err := net.Listen("tcp", a.adminServer.Addr)
	if err != nil {
		return errors.Join(fmt.Errorf("listen admin http %s: %w", a.adminServer.Addr, err), a.closeInfrastructure())
	}
	if a.playerKickBus != nil {
		if err := a.playerKickBus.Start(ctx, a.nodeInfo.ServerID, func(notice iredis.PlayerKickNotice) {
			switch notice.Target {
			case iredis.PlayerKickTargetConnection:
				a.wsServer.KickConnection(notice.UID, notice.ConnID, notice.Reason)
			case iredis.PlayerKickTargetUID:
				a.wsServer.KickUID(context.Background(), notice.UID, notice.Reason)
			case iredis.PlayerKickTargetAll:
				a.wsServer.KickAll(notice.Reason)
			}
		}); err != nil {
			_ = adminListener.Close()
			return errors.Join(err, a.closeInfrastructure())
		}
	}

	if err := a.wsServer.Start(ctx); err != nil {
		_ = adminListener.Close()
		if a.playerKickBus != nil {
			_ = a.playerKickBus.Stop()
		}
		return errors.Join(err, a.closeInfrastructure())
	}
	if err := a.reportNode(ctx); err != nil {
		_ = adminListener.Close()
		_ = a.wsServer.Stop(context.Background())
		if a.playerKickBus != nil {
			_ = a.playerKickBus.Stop()
		}
		return errors.Join(fmt.Errorf("register game server node: %w", err), a.closeInfrastructure())
	}
	a.startNodeHeartbeat(ctx)
	if a.stateMaintainer != nil {
		a.stateMaintainer.Start()
	}

	go func() {
		ilog.Infof("admin http server listening on %s", a.adminServer.Addr)
		if err := a.adminServer.Serve(adminListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("admin http server stopped with error: %v", err)
		}
	}()
	return nil
}

// Stop 先停止节点心跳并注销节点，再关闭连接、状态维护器和基础设施。
func (a *Application) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var firstErr error
	if a.nodeHeartbeatCancel != nil {
		a.nodeHeartbeatCancel()
		a.nodeHeartbeatWG.Wait()
	}
	if a.nodeRegistry != nil {
		if err := a.nodeRegistry.RemoveNode(shutdownCtx, a.nodeInfo.ServerID); err != nil {
			firstErr = err
		}
	}
	if a.playerKickBus != nil {
		if err := a.playerKickBus.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.wsServer.Stop(shutdownCtx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	}
	if a.stateMaintainer != nil {
		if err := a.stateMaintainer.Stop(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.adminServer.Shutdown(shutdownCtx); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := a.closeInfrastructure(); err != nil && firstErr == nil {
		firstErr = err
	}
	ilog.Infof("application stopped node=%s", a.cfg.Server.NodeID)
	return firstErr
}

// closeInfrastructure 关闭 MySQL 和 Redis，并确保单项失败不阻断其他资源释放。
func (a *Application) closeInfrastructure() error {
	var closeErrors []error
	if a.dbPool != nil {
		dbPool := a.dbPool
		a.dbPool = nil
		if err := dbPool.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close mysql: %w", err))
		}
	}
	if a.redisClient != nil {
		redisClient := a.redisClient
		a.redisClient = nil
		if err := redisClient.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close redis: %w", err))
		}
	}
	return errors.Join(closeErrors...)
}

// reportNode 上报节点当前连接数和 drain 状态，供 LoginService 做准入分配。
func (a *Application) reportNode(ctx context.Context) error {
	if a.nodeRegistry == nil {
		return nil
	}
	node := a.nodeInfo
	node.Online = a.wsServer.ConnectionCount()
	node.Drain = a.wsServer.IsDrainMode()
	return a.nodeRegistry.UpsertNode(ctx, node, a.nodeTTL)
}

// startNodeHeartbeat 周期刷新节点 TTL；进程异常退出后记录会自然过期。
func (a *Application) startNodeHeartbeat(ctx context.Context) {
	if a.nodeRegistry == nil || a.nodeHeartbeatInterval <= 0 {
		return
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	a.nodeHeartbeatCancel = cancel
	a.nodeHeartbeatWG.Add(1)
	go func() {
		defer a.nodeHeartbeatWG.Done()
		ticker := time.NewTicker(a.nodeHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if err := a.reportNode(heartbeatCtx); err != nil {
					ilog.Errorf("report game server node failed node=%s err=%v", a.nodeInfo.ServerID, err)
				}
			}
		}
	}()
}
