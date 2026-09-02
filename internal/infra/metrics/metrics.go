package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const latencySampleSize = 2048

type Snapshot struct {
	WSConnections int64 `json:"ws_connections"`
	WSAuthSuccess int64 `json:"ws_auth_success"`
	WSAuthFailed  int64 `json:"ws_auth_failed"`
	WSRateLimited int64 `json:"ws_rate_limited"`
	WSQueueKick   int64 `json:"ws_queue_kick"`
	WSBizRequests int64 `json:"ws_biz_requests"`
	WSBizP95MS    int64 `json:"ws_biz_duration_p95_ms"`
	WSBizP99MS    int64 `json:"ws_biz_duration_p99_ms"`
	DBRequests    int64 `json:"db_requests"`
	DBP95MS       int64 `json:"db_duration_p95_ms"`
	DBP99MS       int64 `json:"db_duration_p99_ms"`
	RedisRequests int64 `json:"redis_requests"`
	RedisP95MS    int64 `json:"redis_duration_p95_ms"`
	RedisP99MS    int64 `json:"redis_duration_p99_ms"`
}

type Registry struct {
	wsConnections atomic.Int64
	wsAuthSuccess atomic.Int64
	wsAuthFailed  atomic.Int64
	wsRateLimited atomic.Int64
	wsQueueKick   atomic.Int64
	wsBizRequests atomic.Int64
	dbRequests    atomic.Int64
	redisRequests atomic.Int64

	wsBizLatency durationSamples
	dbLatency    durationSamples
	redisLatency durationSamples
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) SetWSConnections(v int64) { r.wsConnections.Store(v) }
func (r *Registry) IncWSAuthSuccess()        { r.wsAuthSuccess.Add(1) }
func (r *Registry) IncWSAuthFailed()         { r.wsAuthFailed.Add(1) }
func (r *Registry) IncWSRateLimited()        { r.wsRateLimited.Add(1) }
func (r *Registry) IncWSQueueKick()          { r.wsQueueKick.Add(1) }

// ObserveWSBizDuration 记录服务端业务处理耗时，包含玩家分片排队和 Handler 执行时间。
func (r *Registry) ObserveWSBizDuration(duration time.Duration) {
	r.wsBizLatency.observe(duration)
	r.wsBizRequests.Add(1)
}

// ObserveDBDuration 记录一条数据库操作的执行耗时。
func (r *Registry) ObserveDBDuration(duration time.Duration) {
	r.dbLatency.observe(duration)
	r.dbRequests.Add(1)
}

// ObserveRedisDuration 记录一次 Redis 命令或 Pipeline 的执行耗时。
func (r *Registry) ObserveRedisDuration(duration time.Duration) {
	r.redisLatency.observe(duration)
	r.redisRequests.Add(1)
}

func (r *Registry) Snapshot() Snapshot {
	wsP95, wsP99 := r.wsBizLatency.percentiles()
	dbP95, dbP99 := r.dbLatency.percentiles()
	redisP95, redisP99 := r.redisLatency.percentiles()
	return Snapshot{
		WSConnections: r.wsConnections.Load(),
		WSAuthSuccess: r.wsAuthSuccess.Load(),
		WSAuthFailed:  r.wsAuthFailed.Load(),
		WSRateLimited: r.wsRateLimited.Load(),
		WSQueueKick:   r.wsQueueKick.Load(),
		WSBizRequests: r.wsBizRequests.Load(),
		WSBizP95MS:    wsP95,
		WSBizP99MS:    wsP99,
		DBRequests:    r.dbRequests.Load(),
		DBP95MS:       dbP95,
		DBP99MS:       dbP99,
		RedisRequests: r.redisRequests.Load(),
		RedisP95MS:    redisP95,
		RedisP99MS:    redisP99,
	}
}

type durationSamples struct {
	mu     sync.Mutex
	values [latencySampleSize]int64
	count  int
	next   int
}

func (s *durationSamples) observe(duration time.Duration) {
	milliseconds := duration.Milliseconds()
	if milliseconds < 0 {
		milliseconds = 0
	}

	s.mu.Lock()
	s.values[s.next] = milliseconds
	s.next = (s.next + 1) % latencySampleSize
	if s.count < latencySampleSize {
		s.count++
	}
	s.mu.Unlock()
}

func (s *durationSamples) percentiles() (int64, int64) {
	s.mu.Lock()
	values := append([]int64(nil), s.values[:s.count]...)
	s.mu.Unlock()
	if len(values) == 0 {
		return 0, 0
	}

	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return percentile(values, 95), percentile(values, 99)
}

func percentile(sortedValues []int64, percent int) int64 {
	rank := (len(sortedValues)*percent + 99) / 100
	if rank < 1 {
		rank = 1
	}
	return sortedValues[rank-1]
}
