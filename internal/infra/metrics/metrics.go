package metrics

import "sync/atomic"

type Snapshot struct {
	WSConnections  int64 `json:"ws_connections"`
	WSAuthSuccess  int64 `json:"ws_auth_success"`
	WSAuthFailed   int64 `json:"ws_auth_failed"`
	WSRateLimited  int64 `json:"ws_rate_limited"`
	WSQueueDrop    int64 `json:"ws_queue_drop"`
	WSQueueKick    int64 `json:"ws_queue_kick"`
	FlushEnqueued  int64 `json:"flush_enqueued"`
	FlushQueueLen  int64 `json:"flush_queue_len"`
	FlushProcessed int64 `json:"flush_processed"`
	FlushSaved     int64 `json:"flush_saved"`
	FlushFailed    int64 `json:"flush_failed"`
	FlushRetried   int64 `json:"flush_retried"`
	FlushDropped   int64 `json:"flush_dropped"`
}

type Registry struct {
	wsConnections atomic.Int64
	wsAuthSuccess atomic.Int64
	wsAuthFailed  atomic.Int64
	wsRateLimited atomic.Int64
	wsQueueDrop   atomic.Int64
	wsQueueKick   atomic.Int64

	flushEnqueued  atomic.Int64
	flushQueueLen  atomic.Int64
	flushProcessed atomic.Int64
	flushSaved     atomic.Int64
	flushFailed    atomic.Int64
	flushRetried   atomic.Int64
	flushDropped   atomic.Int64
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) SetWSConnections(v int64) { r.wsConnections.Store(v) }
func (r *Registry) IncWSAuthSuccess()        { r.wsAuthSuccess.Add(1) }
func (r *Registry) IncWSAuthFailed()         { r.wsAuthFailed.Add(1) }
func (r *Registry) IncWSRateLimited()        { r.wsRateLimited.Add(1) }
func (r *Registry) IncWSQueueDrop()          { r.wsQueueDrop.Add(1) }
func (r *Registry) IncWSQueueKick()          { r.wsQueueKick.Add(1) }

func (r *Registry) IncFlushEnqueued()        { r.flushEnqueued.Add(1) }
func (r *Registry) SetFlushQueueLen(v int64) { r.flushQueueLen.Store(v) }
func (r *Registry) IncFlushProcessed()       { r.flushProcessed.Add(1) }
func (r *Registry) IncFlushSaved()           { r.flushSaved.Add(1) }
func (r *Registry) IncFlushFailed()          { r.flushFailed.Add(1) }
func (r *Registry) IncFlushRetried()         { r.flushRetried.Add(1) }
func (r *Registry) IncFlushDropped()         { r.flushDropped.Add(1) }

func (r *Registry) Snapshot() Snapshot {
	return Snapshot{
		WSConnections:  r.wsConnections.Load(),
		WSAuthSuccess:  r.wsAuthSuccess.Load(),
		WSAuthFailed:   r.wsAuthFailed.Load(),
		WSRateLimited:  r.wsRateLimited.Load(),
		WSQueueDrop:    r.wsQueueDrop.Load(),
		WSQueueKick:    r.wsQueueKick.Load(),
		FlushEnqueued:  r.flushEnqueued.Load(),
		FlushQueueLen:  r.flushQueueLen.Load(),
		FlushProcessed: r.flushProcessed.Load(),
		FlushSaved:     r.flushSaved.Load(),
		FlushFailed:    r.flushFailed.Load(),
		FlushRetried:   r.flushRetried.Load(),
		FlushDropped:   r.flushDropped.Load(),
	}
}
