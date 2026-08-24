package ws

import (
	"testing"
	"time"
)

func TestRateLimiterDeleteAllowsKeyAgain(t *testing.T) {
	limiter := NewRateLimiter(time.Hour)
	const key = "conn-1:biz"

	if !limiter.Allow(key) {
		t.Fatal("first request should be allowed")
	}
	if limiter.Allow(key) {
		t.Fatal("second request within min gap should be rejected")
	}

	limiter.Delete(key)
	if !limiter.Allow(key) {
		t.Fatal("request should be allowed after rate limit record is deleted")
	}
}
