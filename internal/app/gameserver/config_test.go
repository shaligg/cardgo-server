package gameserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProcessConfigs(t *testing.T) {
	for _, env := range []string{"local", "staging", "prod"} {
		t.Run(env, func(t *testing.T) {
			path := "../../../configs/gameserver." + env + ".yaml"
			t.Setenv("GAME_CONFIG", path)
			cfg, err := LoadConfigFromEnv()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Server.AdminPort != 8082 || cfg.Server.WSPort != 8081 || cfg.Server.MaxConnections != 2000 {
				t.Fatalf("unexpected game server config: %+v", cfg.Server)
			}
			if cfg.Auth.Issuer != "login-module" || cfg.Auth.SecretEnvKey != "GAME_TICKET_SECRET" || cfg.DB.DSNEnvKey != "GAME_DB_DSN" {
				t.Fatal("missing ticket or database config")
			}
		})
	}
	if _, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Fatal("missing config should fail")
	}
}

func TestLoadConfigRejectsInvalidConfig(t *testing.T) {
	data, err := os.ReadFile("../../../configs/gameserver.local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, old, replacement string }{
		{"unknown field", "server:", "server:\n  unknown_setting: true"},
		{"node id", `node_id: "node-a"`, `node_id: ""`},
		{"admin port", "admin_port: 8082", "admin_port: 0"},
		{"ws port", "ws_port: 8081", "ws_port: 70000"},
		{"advertised ws address", `advertised_ws_addr: "ws://127.0.0.1:8081/ws"`, `advertised_ws_addr: "http://127.0.0.1:8081/ws"`},
		{"max connections", "max_connections: 2000", "max_connections: 0"},
		{"auth issuer", `issuer: "login-module"`, `issuer: ""`},
		{"auth algorithm", `algorithm: "hmac-sha256"`, `algorithm: "none"`},
		{"admin token", "require_auth: false\n  token_env_key: \"GAME_ADMIN_TOKEN\"", "require_auth: true\n  token_env_key: \"\""},
		{"pong wait", "pong_wait_sec: 60", "pong_wait_sec: 30"},
		{"send queue", "send_queue_size: 256", "send_queue_size: 0"},
		{"db pool", "max_idle_conns: 10", "max_idle_conns: 30"},
		{"owner ttl", "owner_ttl_sec: 120", "owner_ttl_sec: 5"},
		{"redis address", `addr: "127.0.0.1:6379"`, `addr: ""`},
		{"node ttl", "node_ttl_sec: 15", "node_ttl_sec: 5"},
		{"web search url", `base_url: "https://zh.wikipedia.org/w/api.php"`, `base_url: ""`},
		{"game data path", `item_config_path: "configs/gamedata/items.json"`, `item_config_path: ""`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			invalid := strings.Replace(string(data), tt.old, tt.replacement, 1)
			if invalid == string(data) {
				t.Fatalf("test replacement %q was not found", tt.old)
			}
			if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(path); err == nil {
				t.Fatal("invalid config should fail")
			}
		})
	}
}
