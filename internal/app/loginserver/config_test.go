package loginserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProcessConfigs(t *testing.T) {
	for _, env := range []string{"local", "staging", "prod"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("GAME_CONFIG", "../../../configs/loginserver."+env+".yaml")
			cfg, err := LoadConfigFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.HTTP.Port != 8080 || cfg.Auth.Issuer != "login-module" || cfg.Auth.TicketTTLSec != 60 || cfg.Auth.SecretEnvKey != "GAME_TICKET_SECRET" {
				t.Fatalf("unexpected login config: %+v", cfg)
			}
			if cfg.Redis.NodeKeyPrefix != "game:gameserver" || cfg.Redis.PlayerOwnerKeyPrefix != "game:player_owner" {
				t.Fatal("unexpected shared redis prefixes")
			}
		})
	}
}

func TestLoadConfigRejectsInvalidConfig(t *testing.T) {
	data, err := os.ReadFile("../../../configs/loginserver.local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, old, replacement string }{
		{"host", `host: "0.0.0.0"`, `host: ""`},
		{"port", "port: 8080", "port: 0"},
		{"issuer", `issuer: "login-module"`, `issuer: ""`},
		{"algorithm", `algorithm: "hmac-sha256"`, `algorithm: "none"`},
		{"ttl", "ticket_ttl_sec: 60", "ticket_ttl_sec: 0"},
		{"secret key", `secret_env_key: "GAME_TICKET_SECRET"`, `secret_env_key: ""`},
		{"redis address", `addr: "127.0.0.1:6379"`, `addr: ""`},
		{"redis db", "db: 0", "db: -1"},
		{"node prefix", `node_key_prefix: "game:gameserver"`, `node_key_prefix: ""`},
		{"owner prefix", `player_owner_key_prefix: "game:player_owner"`, `player_owner_key_prefix: ""`},
		{"unknown game config", "http:", "node_id: node-a\nhttp:"},
		{"missing auth", "auth:", "unused_auth:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(string(data), tt.old, tt.replacement, 1)), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); err == nil {
				t.Fatal("invalid config should fail")
			}
		})
	}
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing config should fail")
	}
}
