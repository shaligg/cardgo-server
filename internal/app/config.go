package app

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/config.local.yaml"

type Config struct {
	Server struct {
		NodeID           string `yaml:"node_id"`
		APIHost          string `yaml:"api_host"`
		APIPort          int    `yaml:"api_port"`
		WSHost           string `yaml:"ws_host"`
		WSPort           int    `yaml:"ws_port"`
		MaxConnections   int    `yaml:"max_connections"`
		DrainMode        bool   `yaml:"drain_mode"`
		DispatcherShards int    `yaml:"dispatcher_shards"`
		FlushQueueMax    int    `yaml:"flush_queue_max"`
		FlushMaxRetry    int    `yaml:"flush_max_retry"`
	} `yaml:"server"`
	Auth struct {
		Issuer       string `yaml:"issuer"`
		Algorithm    string `yaml:"algorithm"`
		TicketTTLSec int    `yaml:"ticket_ttl_sec"`
		NonceTTLSec  int    `yaml:"nonce_ttl_sec"`
		SecretEnvKey string `yaml:"secret_env_key"`
	} `yaml:"auth"`
	WS struct {
		HeartbeatIntervalSec int `yaml:"heartbeat_interval_sec"`
		PongWaitSec          int `yaml:"pong_wait_sec"`
		WriteWaitSec         int `yaml:"write_wait_sec"`
		SendQueueSize        int `yaml:"send_queue_size"`
		BizMinGapMS          int `yaml:"biz_min_gap_ms"`
		MaxMessageBytes      int `yaml:"max_message_bytes"`
	} `yaml:"ws"`
	DB struct {
		DSN string `yaml:"dsn"`
	} `yaml:"db"`
	Cache struct {
		L1TTLSec int `yaml:"l1_ttl_sec"`
		L2TTLSec int `yaml:"l2_ttl_sec"`
	} `yaml:"cache"`
	Debug struct {
		EnableWSDebugOps bool `yaml:"enable_ws_debug_ops"`
	} `yaml:"debug"`
	GameData struct {
		ItemConfigPath     string `yaml:"item_config_path"`
		CardConfigPath     string `yaml:"card_config_path"`
		OrderConfigPath    string `yaml:"order_config_path"`
		LevelConfigPath    string `yaml:"level_config_path"`
		FacilityConfigPath string `yaml:"facility_config_path"`
	} `yaml:"gamedata"`
}

func LoadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func LoadConfigFromEnv() (Config, error) {
	return LoadConfig(os.Getenv("GAME_CONFIG"))
}

func defaultConfig() Config {
	var cfg Config
	cfg.Server.NodeID = "node-a"
	cfg.Server.APIHost = "0.0.0.0"
	cfg.Server.APIPort = 8080
	cfg.Server.WSHost = "0.0.0.0"
	cfg.Server.WSPort = 8081
	cfg.Server.MaxConnections = 2000
	cfg.Server.DispatcherShards = 64
	cfg.Server.FlushQueueMax = 10000
	cfg.Server.FlushMaxRetry = 3

	cfg.Auth.Issuer = "login-module"
	cfg.Auth.Algorithm = "hmac-sha256"
	cfg.Auth.TicketTTLSec = 60
	cfg.Auth.NonceTTLSec = 120
	cfg.Auth.SecretEnvKey = "GAME_TICKET_SECRET"

	cfg.WS.HeartbeatIntervalSec = 30
	cfg.WS.PongWaitSec = 60
	cfg.WS.WriteWaitSec = 10
	cfg.WS.SendQueueSize = 256
	cfg.WS.BizMinGapMS = 5
	cfg.WS.MaxMessageBytes = 64 * 1024
	cfg.DB.DSN = "file:game_demo.db?cache=shared&_busy_timeout=5000"
	cfg.Cache.L1TTLSec = 30
	cfg.Cache.L2TTLSec = 300
	cfg.Debug.EnableWSDebugOps = true
	cfg.GameData.ItemConfigPath = "configs/gamedata/items.json"
	cfg.GameData.CardConfigPath = "configs/gamedata/cards.json"
	cfg.GameData.OrderConfigPath = "configs/gamedata/orders.json"
	cfg.GameData.LevelConfigPath = "configs/gamedata/levels.json"
	cfg.GameData.FacilityConfigPath = "configs/gamedata/facilities.json"
	return cfg
}

func applyDefaults(cfg *Config) {
	if cfg.Server.NodeID == "" {
		cfg.Server.NodeID = "node-a"
	}
	if cfg.Server.APIHost == "" {
		cfg.Server.APIHost = "0.0.0.0"
	}
	if cfg.Server.APIPort == 0 {
		cfg.Server.APIPort = 8080
	}
	if cfg.Server.WSHost == "" {
		cfg.Server.WSHost = "0.0.0.0"
	}
	if cfg.Server.WSPort == 0 {
		cfg.Server.WSPort = 8081
	}
	if cfg.Server.MaxConnections <= 0 {
		cfg.Server.MaxConnections = 2000
	}
	if cfg.Server.DispatcherShards <= 0 {
		cfg.Server.DispatcherShards = 64
	}
	if cfg.Server.FlushQueueMax <= 0 {
		cfg.Server.FlushQueueMax = 10000
	}
	if cfg.Server.FlushMaxRetry < 0 {
		cfg.Server.FlushMaxRetry = 0
	}
	if cfg.Auth.TicketTTLSec <= 0 {
		cfg.Auth.TicketTTLSec = 60
	}
	if cfg.Auth.NonceTTLSec <= 0 {
		cfg.Auth.NonceTTLSec = 120
	}
	if cfg.WS.HeartbeatIntervalSec <= 0 {
		cfg.WS.HeartbeatIntervalSec = 30
	}
	if cfg.WS.PongWaitSec <= 0 {
		cfg.WS.PongWaitSec = 60
	}
	if cfg.WS.WriteWaitSec <= 0 {
		cfg.WS.WriteWaitSec = 10
	}
	if cfg.WS.SendQueueSize <= 0 {
		cfg.WS.SendQueueSize = 256
	}
	if cfg.WS.BizMinGapMS < 0 {
		cfg.WS.BizMinGapMS = 0
	}
	if cfg.WS.MaxMessageBytes <= 0 {
		cfg.WS.MaxMessageBytes = 64 * 1024
	}
	if cfg.DB.DSN == "" {
		cfg.DB.DSN = "file:game_demo.db?cache=shared&_busy_timeout=5000"
	}
	if cfg.Cache.L1TTLSec <= 0 {
		cfg.Cache.L1TTLSec = 30
	}
	if cfg.Cache.L2TTLSec <= 0 {
		cfg.Cache.L2TTLSec = 300
	}
	if cfg.GameData.ItemConfigPath == "" {
		cfg.GameData.ItemConfigPath = "configs/gamedata/items.json"
	}
	if cfg.GameData.CardConfigPath == "" {
		cfg.GameData.CardConfigPath = "configs/gamedata/cards.json"
	}
	if cfg.GameData.OrderConfigPath == "" {
		cfg.GameData.OrderConfigPath = "configs/gamedata/orders.json"
	}
	if cfg.GameData.LevelConfigPath == "" {
		cfg.GameData.LevelConfigPath = "configs/gamedata/levels.json"
	}
	if cfg.GameData.FacilityConfigPath == "" {
		cfg.GameData.FacilityConfigPath = "configs/gamedata/facilities.json"
	}
}
