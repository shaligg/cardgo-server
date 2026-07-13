package gameserver

import (
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

type flushMetricsObserver struct {
	reg *metrics.Registry
}

func newFlushMetricsObserver(reg *metrics.Registry) state.FlushObserver {
	if reg == nil {
		return nil
	}
	return &flushMetricsObserver{reg: reg}
}

func (o *flushMetricsObserver) OnQueueLen(length int) { o.reg.SetFlushQueueLen(int64(length)) }
func (o *flushMetricsObserver) OnProcessed()          { o.reg.IncFlushProcessed() }
func (o *flushMetricsObserver) OnSaved()              { o.reg.IncFlushSaved() }
func (o *flushMetricsObserver) OnFailed()             { o.reg.IncFlushFailed() }
func (o *flushMetricsObserver) OnRetried()            { o.reg.IncFlushRetried() }
func (o *flushMetricsObserver) OnDropped()            { o.reg.IncFlushDropped() }
