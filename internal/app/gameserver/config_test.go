package gameserver

import (
	"path/filepath"
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
