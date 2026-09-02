package main

import (
	"testing"

	"github.com/bigfish/go_orm_1/internal/infra/metrics"
)

func TestEvaluateHealthySnapshot(t *testing.T) {
	current := metrics.Snapshot{
		WSConnections: 1000,
		WSAuthSuccess: 10000,
		WSBizRequests: 10000,
		WSBizP95MS:    20,
		WSBizP99MS:    60,
		DBRequests:    1000,
		DBP95MS:       5,
		RedisRequests: 1000,
		RedisP95MS:    2,
	}
	if alerts := evaluate(current, nil, defaultThresholds()); len(alerts) != 0 {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
}

func TestEvaluateThresholdsAndCounterDeltas(t *testing.T) {
	previous := metrics.Snapshot{
		WSAuthSuccess: 1000,
		WSAuthFailed:  1,
		WSQueueKick:   3,
	}
	current := metrics.Snapshot{
		WSConnections: 2000,
		WSAuthSuccess: 1090,
		WSAuthFailed:  2,
		WSQueueKick:   4,
		WSBizRequests: 10,
		WSBizP95MS:    50,
		WSBizP99MS:    120,
		DBRequests:    10,
		DBP95MS:       20,
		RedisRequests: 10,
		RedisP95MS:    5,
	}

	alerts := evaluate(current, &previous, defaultThresholds())
	if len(alerts) != 7 {
		t.Fatalf("alert count = %d, want 7: %+v", len(alerts), alerts)
	}
	if alerts[0].level != "CRITICAL" {
		t.Fatalf("first alert = %+v, want critical connection alert", alerts[0])
	}
}

func TestCounterDeltaHandlesProcessRestart(t *testing.T) {
	if got := counterDelta(3, 100); got != 3 {
		t.Fatalf("counterDelta = %d, want 3", got)
	}
}
