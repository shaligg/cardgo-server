package session

import (
	"context"
	"testing"
)

func TestMemoryLastServerStoreSaveGetDelete(t *testing.T) {
	store := NewMemoryLastServerStore()
	ctx := context.Background()

	if _, ok, err := store.GetLastServerID(ctx, "u1"); err != nil || ok {
		t.Fatalf("expected empty store miss, ok=%v err=%v", ok, err)
	}

	if err := store.SaveLastServerID(ctx, "u1", "gs-a"); err != nil {
		t.Fatalf("SaveLastServerID returned error: %v", err)
	}
	serverID, ok, err := store.GetLastServerID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetLastServerID returned error: %v", err)
	}
	if !ok || serverID != "gs-a" {
		t.Fatalf("expected gs-a, got %q ok=%v", serverID, ok)
	}

	if err := store.DeleteLastServerID(ctx, "u1"); err != nil {
		t.Fatalf("DeleteLastServerID returned error: %v", err)
	}
	if _, ok, err := store.GetLastServerID(ctx, "u1"); err != nil || ok {
		t.Fatalf("expected deleted store miss, ok=%v err=%v", ok, err)
	}
}

func TestMemoryLastServerStoreOverwrite(t *testing.T) {
	store := NewMemoryLastServerStore()
	ctx := context.Background()

	_ = store.SaveLastServerID(ctx, "u1", "gs-a")
	_ = store.SaveLastServerID(ctx, "u1", "gs-b")

	serverID, ok, err := store.GetLastServerID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetLastServerID returned error: %v", err)
	}
	if !ok || serverID != "gs-b" {
		t.Fatalf("expected overwrite to gs-b, got %q ok=%v", serverID, ok)
	}
}

func TestMemoryLastServerStoreIgnoresEmptyInput(t *testing.T) {
	store := NewMemoryLastServerStore()
	ctx := context.Background()

	if err := store.SaveLastServerID(ctx, "", "gs-a"); err != nil {
		t.Fatalf("SaveLastServerID empty uid returned error: %v", err)
	}
	if err := store.SaveLastServerID(ctx, "u1", ""); err != nil {
		t.Fatalf("SaveLastServerID empty server returned error: %v", err)
	}

	if _, ok, err := store.GetLastServerID(ctx, ""); err != nil || ok {
		t.Fatalf("expected empty uid miss, ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.GetLastServerID(ctx, "u1"); err != nil || ok {
		t.Fatalf("expected empty server save ignored, ok=%v err=%v", ok, err)
	}
}
