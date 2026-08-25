package session

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCommandCacheKeepsOnlyRecentResults(t *testing.T) {
	cache := NewCommandCache(time.Minute, 2, 1024)
	for _, reqID := range []string{"r1", "r2", "r3"} {
		cache.Put("u1", reqID, CommandResult{OpCode: 1, Result: json.RawMessage(`{"ok":true}`)})
	}
	if _, ok := cache.Get("u1", "r1"); ok {
		t.Fatal("oldest result was not evicted")
	}
	if _, ok := cache.Get("u1", "r2"); !ok {
		t.Fatal("recent result r2 is missing")
	}
	if _, ok := cache.Get("u1", "r3"); !ok {
		t.Fatal("recent result r3 is missing")
	}
}

func TestCommandCacheRejectsOversizedResultAndCleansExpired(t *testing.T) {
	cache := NewCommandCache(time.Minute, 10, 4)
	if cache.Put("u1", "large", CommandResult{Result: json.RawMessage(`{"large":true}`)}) {
		t.Fatal("oversized result was cached")
	}
	if !cache.Put("u1", "small", CommandResult{Result: json.RawMessage(`null`)}) {
		t.Fatal("small result was not cached")
	}
	cache.CleanupExpired(time.Now().Add(2 * time.Minute))
	if _, ok := cache.Get("u1", "small"); ok {
		t.Fatal("expired result was not cleaned")
	}
}
