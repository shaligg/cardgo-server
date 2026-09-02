package redis

import (
	"context"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	goredis "github.com/redis/go-redis/v9"
)

func TestMetricsHookCollectsCommandAndPipelineMetrics(t *testing.T) {
	registry := metrics.NewRegistry()
	hook := metricsHook{registry: registry}

	process := hook.ProcessHook(func(context.Context, goredis.Cmder) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	if err := process(context.Background(), nil); err != nil {
		t.Fatalf("process returned error: %v", err)
	}

	processPipeline := hook.ProcessPipelineHook(func(context.Context, []goredis.Cmder) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	if err := processPipeline(context.Background(), nil); err != nil {
		t.Fatalf("pipeline returned error: %v", err)
	}

	snapshot := registry.Snapshot()
	if snapshot.RedisRequests != 2 {
		t.Fatalf("redis_requests = %d, want 2", snapshot.RedisRequests)
	}
	if snapshot.RedisP95MS < 1 || snapshot.RedisP99MS < 1 {
		t.Fatalf("unexpected Redis percentiles: p95=%d p99=%d", snapshot.RedisP95MS, snapshot.RedisP99MS)
	}
}
