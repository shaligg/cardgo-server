package metrics

import (
	"testing"
	"time"
)

func TestWSBizDurationPercentiles(t *testing.T) {
	registry := NewRegistry()
	for milliseconds := 1; milliseconds <= 100; milliseconds++ {
		registry.ObserveWSBizDuration(time.Duration(milliseconds) * time.Millisecond)
	}

	snapshot := registry.Snapshot()
	if snapshot.WSBizRequests != 100 {
		t.Fatalf("ws_biz_requests = %d, want 100", snapshot.WSBizRequests)
	}
	if snapshot.WSBizP95MS != 95 {
		t.Fatalf("ws_biz_duration_p95_ms = %d, want 95", snapshot.WSBizP95MS)
	}
	if snapshot.WSBizP99MS != 99 {
		t.Fatalf("ws_biz_duration_p99_ms = %d, want 99", snapshot.WSBizP99MS)
	}
}

func TestWSBizDurationSnapshotIsEmptyBeforeObservation(t *testing.T) {
	snapshot := NewRegistry().Snapshot()
	if snapshot.WSBizRequests != 0 || snapshot.WSBizP95MS != 0 || snapshot.WSBizP99MS != 0 ||
		snapshot.DBRequests != 0 || snapshot.RedisRequests != 0 {
		t.Fatalf("unexpected empty snapshot: %+v", snapshot)
	}
}

func TestDBAndRedisDurationPercentiles(t *testing.T) {
	registry := NewRegistry()
	for milliseconds := 1; milliseconds <= 100; milliseconds++ {
		duration := time.Duration(milliseconds) * time.Millisecond
		registry.ObserveDBDuration(duration)
		registry.ObserveRedisDuration(duration)
	}

	snapshot := registry.Snapshot()
	if snapshot.DBRequests != 100 || snapshot.DBP95MS != 95 || snapshot.DBP99MS != 99 {
		t.Fatalf("unexpected DB metrics: requests=%d p95=%d p99=%d", snapshot.DBRequests, snapshot.DBP95MS, snapshot.DBP99MS)
	}
	if snapshot.RedisRequests != 100 || snapshot.RedisP95MS != 95 || snapshot.RedisP99MS != 99 {
		t.Fatalf("unexpected Redis metrics: requests=%d p95=%d p99=%d", snapshot.RedisRequests, snapshot.RedisP95MS, snapshot.RedisP99MS)
	}
}
