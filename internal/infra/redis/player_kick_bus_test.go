package redis

import "testing"

func TestPlayerKickChannelUsesTargetServer(t *testing.T) {
	bus := NewPlayerKickBus(nil, "game:gameserver:kick")
	if channel := bus.channel("gs-a"); channel != "game:gameserver:kick:gs-a" {
		t.Fatalf("channel = %s", channel)
	}
	if channel := bus.broadcastChannel(); channel != "game:gameserver:kick:all" {
		t.Fatalf("broadcast channel = %s", channel)
	}
}

func TestValidatePlayerKickNoticeTargets(t *testing.T) {
	if err := validatePlayerKickNotice(PlayerKickNotice{
		Target: PlayerKickTargetConnection,
		UID:    "u1",
		ConnID: "conn-a",
	}); err != nil {
		t.Fatalf("connection target: %v", err)
	}
	if err := validatePlayerKickNotice(PlayerKickNotice{
		Target: PlayerKickTargetUID,
		UID:    "u1",
	}); err != nil {
		t.Fatalf("uid target: %v", err)
	}
	if err := validatePlayerKickNotice(PlayerKickNotice{Target: PlayerKickTargetAll}); err != nil {
		t.Fatalf("all target: %v", err)
	}
	if err := validatePlayerKickNotice(PlayerKickNotice{
		Target: PlayerKickTargetConnection,
		UID:    "u1",
	}); err == nil {
		t.Fatal("connection target should require conn_id")
	}
}
