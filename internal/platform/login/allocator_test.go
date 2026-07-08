package login

import (
	"context"
	"errors"
	"testing"
)

type fakeLastServerStore struct {
	serverID string
	ok       bool
	err      error
}

func (s fakeLastServerStore) GetLastServerID(ctx context.Context, uid string) (string, bool, error) {
	_ = ctx
	_ = uid
	return s.serverID, s.ok, s.err
}

func TestRegistryNodeAllocatorPrefersLastServer(t *testing.T) {
	allocator := RegistryNodeAllocator{
		Registry: StaticNodeRegistry{Nodes: []NodeInfo{
			{ServerID: "gs-a", WSAddr: "ws://a/ws", Online: 100, MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-b", WSAddr: "ws://b/ws", Online: 10, MaxOnline: 2000, Healthy: true},
		}},
		LastServer: fakeLastServerStore{serverID: "gs-a", ok: true},
	}

	serverID, wsAddr, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if err != nil {
		t.Fatalf("Allocate returned error: %v", err)
	}
	if serverID != "gs-a" || wsAddr != "ws://a/ws" {
		t.Fatalf("expected last server gs-a, got %s %s", serverID, wsAddr)
	}
}

func TestRegistryNodeAllocatorSkipsUnavailableLastServer(t *testing.T) {
	allocator := RegistryNodeAllocator{
		Registry: StaticNodeRegistry{Nodes: []NodeInfo{
			{ServerID: "gs-a", WSAddr: "ws://a/ws", Online: 2000, MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-b", WSAddr: "ws://b/ws", Online: 100, MaxOnline: 2000, Healthy: true},
		}},
		LastServer: fakeLastServerStore{serverID: "gs-a", ok: true},
	}

	serverID, wsAddr, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if err != nil {
		t.Fatalf("Allocate returned error: %v", err)
	}
	if serverID != "gs-b" || wsAddr != "ws://b/ws" {
		t.Fatalf("expected fallback server gs-b, got %s %s", serverID, wsAddr)
	}
}

func TestRegistryNodeAllocatorPicksLowestLoad(t *testing.T) {
	allocator := RegistryNodeAllocator{
		Registry: StaticNodeRegistry{Nodes: []NodeInfo{
			{ServerID: "gs-a", WSAddr: "ws://a/ws", Online: 1000, MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-b", WSAddr: "ws://b/ws", Online: 100, MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-c", WSAddr: "ws://c/ws", Online: 1, MaxOnline: 2000, Healthy: true, Drain: true},
		}},
	}

	serverID, wsAddr, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if err != nil {
		t.Fatalf("Allocate returned error: %v", err)
	}
	if serverID != "gs-b" || wsAddr != "ws://b/ws" {
		t.Fatalf("expected lowest available server gs-b, got %s %s", serverID, wsAddr)
	}
}

func TestRegistryNodeAllocatorNoAvailableNode(t *testing.T) {
	allocator := RegistryNodeAllocator{
		Registry: StaticNodeRegistry{Nodes: []NodeInfo{
			{ServerID: "gs-a", WSAddr: "ws://a/ws", Online: 2000, MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-b", WSAddr: "ws://b/ws", Online: 10, MaxOnline: 2000, Healthy: false},
		}},
	}

	_, _, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if !errors.Is(err, ErrNoAvailableNode) {
		t.Fatalf("expected ErrNoAvailableNode, got %v", err)
	}
}

func TestRegistryNodeAllocatorReturnsLastServerStoreError(t *testing.T) {
	wantErr := errors.New("last server unavailable")
	allocator := RegistryNodeAllocator{
		Registry:   StaticNodeRegistry{Nodes: []NodeInfo{{ServerID: "gs-a", WSAddr: "ws://a/ws", Healthy: true}}},
		LastServer: fakeLastServerStore{err: wantErr},
	}

	_, _, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected last server error, got %v", err)
	}
}
