package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestMemoryNonceStoreRejectsReplayAndAllowsExpiredNonce(t *testing.T) {
	store := NewMemoryNonceStore()
	ctx := context.Background()
	if err := store.ConsumeOnce(ctx, "nonce-1", time.Minute); err != nil {
		t.Fatalf("first consume returned error: %v", err)
	}
	if err := store.ConsumeOnce(ctx, "nonce-1", time.Minute); !errors.Is(err, ErrReplay) {
		t.Fatalf("replay error = %v, want ErrReplay", err)
	}

	store.items["nonce-1"] = time.Now().Add(-time.Second)
	if err := store.ConsumeOnce(ctx, "nonce-1", time.Minute); err != nil {
		t.Fatalf("consume after expiration returned error: %v", err)
	}
}

func TestMemoryNonceStoreCleansExpiredItemsAtThreshold(t *testing.T) {
	store := NewMemoryNonceStore()
	for i := 0; i < nonceCleanupThreshold-1; i++ {
		store.items[fmt.Sprintf("expired-%d", i)] = time.Now().Add(-time.Second)
	}

	if err := store.ConsumeOnce(context.Background(), "current", time.Minute); err != nil {
		t.Fatalf("ConsumeOnce returned error: %v", err)
	}
	if len(store.items) != 1 {
		t.Fatalf("items = %d, want only current nonce", len(store.items))
	}
	if store.nextCleanup.IsZero() {
		t.Fatal("next cleanup time was not updated")
	}
}
