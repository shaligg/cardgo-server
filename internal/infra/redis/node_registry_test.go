package redis

import "testing"

func TestDecodeNodeInfo(t *testing.T) {
	node, err := decodeNodeInfo(map[string]string{
		"server_id":  "gs-a",
		"ws_addr":    "ws://gs-a/ws",
		"online":     "120",
		"max_online": "2000",
		"healthy":    "1",
		"drain":      "0",
		"region":     "cn",
	})
	if err != nil {
		t.Fatalf("decodeNodeInfo returned error: %v", err)
	}
	if node.ServerID != "gs-a" || node.Online != 120 || node.MaxOnline != 2000 || !node.Healthy || node.Drain {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestPlayerOwnerKeyUsesConfiguredPrefix(t *testing.T) {
	store := NewPlayerOwnerStore(nil, "game:player_owner")
	if key := store.key("u1"); key != "game:player_owner:u1" {
		t.Fatalf("key = %s", key)
	}
}
