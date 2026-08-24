package cache

import (
	"testing"
	"time"
)

func TestL1CacheGetDeletesExpiredItem(t *testing.T) {
	cache := NewL1Cache()
	cache.Set("player:u1", "old", -time.Second)

	if value, ok := cache.Get("player:u1"); ok || value != nil {
		t.Fatalf("expired value = %v, ok = %v; want nil, false", value, ok)
	}

	cache.mu.RLock()
	_, exists := cache.items["player:u1"]
	cache.mu.RUnlock()
	if exists {
		t.Fatal("expired item was not deleted")
	}
}

func TestL1CacheGetKeepsFreshItem(t *testing.T) {
	cache := NewL1Cache()
	cache.Set("player:u1", "fresh", time.Minute)

	value, ok := cache.Get("player:u1")
	if !ok || value != "fresh" {
		t.Fatalf("value = %v, ok = %v; want fresh, true", value, ok)
	}
}
