package gameserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
)

// Start 按 API 占位、WS 监听、节点注册的顺序启动应用，避免注册尚未就绪的节点。
func (a *Application) Start(ctx context.Context) error {
	apiListener, err := net.Listen("tcp", a.apiServer.Addr)
	if err != nil {
		return fmt.Errorf("listen api %s: %w", a.apiServer.Addr, err)
	}

	if err := a.wsServer.Start(ctx); err != nil {
		_ = apiListener.Close()
		return err
	}
	if err := a.reportNode(ctx); err != nil {
		_ = apiListener.Close()
		_ = a.wsServer.Stop(context.Background())
		return fmt.Errorf("register game server node: %w", err)
	}
	a.startNodeHeartbeat(ctx)
	if a.flushWorker != nil {
		a.flushWorker.Start()
	}

	go func() {
		ilog.Infof("api server listening on %s", a.apiServer.Addr)
		if err := a.apiServer.Serve(apiListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("api server stopped with error: %v", err)
		}
	}()
	return nil
}

// Stop 先停止节点心跳并注销节点，再关闭连接、刷盘队列和基础设施。
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
	if err := a.wsServer.Stop(shutdownCtx); err != nil {
		if firstErr == nil {
			firstErr = err
		}
	}
	if a.flushWorker != nil {
		if err := a.flushWorker.Stop(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.apiServer.Shutdown(shutdownCtx); err != nil && firstErr == nil {
		firstErr = err
	}
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	ilog.Infof("application stopped node=%s", a.cfg.Server.NodeID)
	return firstErr
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
