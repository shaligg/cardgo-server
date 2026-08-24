package login

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeLastServerReader struct {
	serverID string
	ok       bool
	err      error
}

type failingNodeRegistry struct {
	err error
}

func (r failingNodeRegistry) ListNodes(context.Context) ([]NodeInfo, error) {
	return nil, r.err
}

func (s fakeLastServerReader) GetLastServerID(ctx context.Context, uid string) (string, bool, error) {
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
		LastServer: fakeLastServerReader{serverID: "gs-a", ok: true},
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
		LastServer: fakeLastServerReader{serverID: "gs-a", ok: true},
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

func TestRegistryNodeAllocatorDistributesEqualLoadNodes(t *testing.T) {
	allocator := RegistryNodeAllocator{
		Registry: StaticNodeRegistry{Nodes: []NodeInfo{
			{ServerID: "gs-a", WSAddr: "ws://a/ws", MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-b", WSAddr: "ws://b/ws", MaxOnline: 2000, Healthy: true},
			{ServerID: "gs-c", WSAddr: "ws://c/ws", MaxOnline: 2000, Healthy: true},
		}},
	}

	selected := make(map[string]bool)
	for i := 0; i < 100; i++ {
		serverID, _, err := allocator.Allocate(context.Background(), fmt.Sprintf("user-%d", i), "127.0.0.1")
		if err != nil {
			t.Fatalf("Allocate returned error: %v", err)
		}
		selected[serverID] = true
	}
	if len(selected) != 3 {
		t.Fatalf("expected equal-load requests to reach all nodes, got %v", selected)
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

func TestRegistryNodeAllocatorReturnsRegistryError(t *testing.T) {
	wantErr := errors.New("redis unavailable")
	allocator := RegistryNodeAllocator{Registry: failingNodeRegistry{err: wantErr}}

	_, _, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected registry error, got %v", err)
	}
}

func TestRegistryNodeAllocatorReturnsLastServerReaderError(t *testing.T) {
	wantErr := errors.New("last server unavailable")
	allocator := RegistryNodeAllocator{
		Registry:   StaticNodeRegistry{Nodes: []NodeInfo{{ServerID: "gs-a", WSAddr: "ws://a/ws", Healthy: true}}},
		LastServer: fakeLastServerReader{err: wantErr},
	}

	_, _, err := allocator.Allocate(context.Background(), "u1", "127.0.0.1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected last server error, got %v", err)
	}
}
