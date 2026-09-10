package state

import (
	"context"
	"sync"
	"time"
)

// OwnerReconciler 校验本机玩家运行时是否仍归属于当前 GameServer。
type OwnerReconciler interface {
	ReconcileOwners(ctx context.Context)
}

// MaintainerOptions 配置跨节点归属核对周期。
type MaintainerOptions struct {
	OwnerCheckInterval time.Duration
	OwnerReconciler    OwnerReconciler
}

// Maintainer 定期核对跨节点玩家归属。
type Maintainer struct {
	ownerCheckInterval time.Duration
	ownerReconciler    OwnerReconciler
	lastOwnerCheck     time.Time
	stopOnce           sync.Once
	stopCh             chan struct{}
	doneCh             chan struct{}
}

// NewMaintainer 创建玩家归属维护器。
func NewMaintainer(opts MaintainerOptions) *Maintainer {
	if opts.OwnerCheckInterval <= 0 {
		opts.OwnerCheckInterval = 5 * time.Second
	}
	return &Maintainer{
		ownerCheckInterval: opts.OwnerCheckInterval,
		ownerReconciler:    opts.OwnerReconciler,
		stopCh:             make(chan struct{}),
		doneCh:             make(chan struct{}),
	}
}

// Start 启动状态维护循环。
func (m *Maintainer) Start() {
	go func() {
		defer close(m.doneCh)
		m.maintain(context.Background(), time.Now())
		ticker := time.NewTicker(m.ownerCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case now := <-ticker.C:
				m.maintain(context.Background(), now)
			}
		}
	}()
}

// Stop 停止状态维护循环并等待退出。
func (m *Maintainer) Stop(ctx context.Context) error {
	m.stopOnce.Do(func() { close(m.stopCh) })
	select {
	case <-m.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Maintainer) maintain(ctx context.Context, now time.Time) {
	if m.ownerReconciler != nil && (m.lastOwnerCheck.IsZero() || now.Sub(m.lastOwnerCheck) >= m.ownerCheckInterval) {
		m.ownerReconciler.ReconcileOwners(ctx)
		m.lastOwnerCheck = now
	}
}
