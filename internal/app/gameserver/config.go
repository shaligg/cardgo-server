package gameserver

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
		AdvertisedWSAddr string `yaml:"advertised_ws_addr"`
		MaxConnections   int    `yaml:"max_connections"`
		DrainMode        bool   `yaml:"drain_mode"`
		DispatcherShards int    `yaml:"dispatcher_shards"`
	} `yaml:"server"`
	Auth struct {
		Issuer       string `yaml:"issuer"`
		Algorithm    string `yaml:"algorithm"`
		TicketTTLSec int    `yaml:"ticket_ttl_sec"`
		NonceTTLSec  int    `yaml:"nonce_ttl_sec"`
		SecretEnvKey string `yaml:"secret_env_key"`
	} `yaml:"auth"`
	Admin struct {
		RequireAuth bool   `yaml:"require_auth"`
		TokenEnvKey string `yaml:"token_env_key"`
	} `yaml:"admin"`
	WS struct {
		HeartbeatIntervalSec int      `yaml:"heartbeat_interval_sec"`
		PongWaitSec          int      `yaml:"pong_wait_sec"`
		WriteWaitSec         int      `yaml:"write_wait_sec"`
		SendQueueSize        int      `yaml:"send_queue_size"`
		BizMinGapMS          int      `yaml:"biz_min_gap_ms"`
		MaxMessageBytes      int      `yaml:"max_message_bytes"`
		AllowedOrigins       []string `yaml:"allowed_origins"`
	} `yaml:"ws"`
	DB struct {
		DSNEnvKey              string `yaml:"dsn_env_key"`
		MaxOpenConns           int    `yaml:"max_open_conns"`
		MaxIdleConns           int    `yaml:"max_idle_conns"`
		ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_sec"`
		ConnMaxIdleTimeSeconds int    `yaml:"conn_max_idle_time_sec"`
	} `yaml:"db"`
	State struct {
		OfflineTTLSec         int `yaml:"offline_ttl_sec"`
		CleanupIntervalSec    int `yaml:"cleanup_interval_sec"`
		OwnerCheckIntervalSec int `yaml:"owner_check_interval_sec"`
		OwnerTTLSec           int `yaml:"owner_ttl_sec"`
	} `yaml:"state"`
	Redis struct {
		Addr                 string `yaml:"addr"`
		PasswordEnvKey       string `yaml:"password_env_key"`
		DB                   int    `yaml:"db"`
		NodeKeyPrefix        string `yaml:"node_key_prefix"`
		PlayerOwnerKeyPrefix string `yaml:"player_owner_key_prefix"`
		NodeHeartbeatSec     int    `yaml:"node_heartbeat_sec"`
		NodeTTLSec           int    `yaml:"node_ttl_sec"`
	} `yaml:"redis"`
	Debug struct {
		EnableWSDebugOps bool `yaml:"enable_ws_debug_ops"`
	} `yaml:"debug"`
	WebSearch struct {
		BaseURL   string `yaml:"base_url"`
		TimeoutMS int    `yaml:"timeout_ms"`
	} `yaml:"web_search"`
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
	cfg.Server.AdvertisedWSAddr = "ws://127.0.0.1:8081/ws"
	cfg.Server.MaxConnections = 2000
	cfg.Server.DispatcherShards = 64

	cfg.Auth.Issuer = "login-module"
	cfg.Auth.Algorithm = "hmac-sha256"
	cfg.Auth.TicketTTLSec = 60
	cfg.Auth.NonceTTLSec = 120
	cfg.Auth.SecretEnvKey = "GAME_TICKET_SECRET"
	cfg.Admin.TokenEnvKey = "GAME_ADMIN_TOKEN"

	cfg.WS.HeartbeatIntervalSec = 30
	cfg.WS.PongWaitSec = 60
	cfg.WS.WriteWaitSec = 10
	cfg.WS.SendQueueSize = 256
	cfg.WS.BizMinGapMS = 5
	cfg.WS.MaxMessageBytes = 64 * 1024
	cfg.DB.DSNEnvKey = "GAME_DB_DSN"
	cfg.DB.MaxOpenConns = 20
	cfg.DB.MaxIdleConns = 10
	cfg.DB.ConnMaxLifetimeSeconds = 1800
	cfg.DB.ConnMaxIdleTimeSeconds = 300
	cfg.State.OfflineTTLSec = 120
	cfg.State.CleanupIntervalSec = 10
	cfg.State.OwnerCheckIntervalSec = 5
	cfg.State.OwnerTTLSec = 120
	cfg.Redis.Addr = "127.0.0.1:6379"
	cfg.Redis.PasswordEnvKey = "GAME_REDIS_PASSWORD"
	cfg.Redis.NodeKeyPrefix = "game:gameserver"
	cfg.Redis.PlayerOwnerKeyPrefix = "game:player_owner"
	cfg.Redis.NodeHeartbeatSec = 5
	cfg.Redis.NodeTTLSec = 15
	cfg.Debug.EnableWSDebugOps = true
	cfg.WebSearch.BaseURL = "https://zh.wikipedia.org/w/api.php"
	cfg.WebSearch.TimeoutMS = 2000
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
	if cfg.Server.AdvertisedWSAddr == "" {
		wsHost := cfg.Server.WSHost
		if wsHost == "" || wsHost == "0.0.0.0" {
			wsHost = "127.0.0.1"
		}
		cfg.Server.AdvertisedWSAddr = fmt.Sprintf("ws://%s:%d/ws", wsHost, cfg.Server.WSPort)
	}
	if cfg.Server.MaxConnections <= 0 {
		cfg.Server.MaxConnections = 2000
	}
	if cfg.Server.DispatcherShards <= 0 {
		cfg.Server.DispatcherShards = 64
	}
	if cfg.Auth.TicketTTLSec <= 0 {
		cfg.Auth.TicketTTLSec = 60
	}
	if cfg.Auth.NonceTTLSec <= 0 {
		cfg.Auth.NonceTTLSec = 120
	}
	if cfg.Admin.TokenEnvKey == "" {
		cfg.Admin.TokenEnvKey = "GAME_ADMIN_TOKEN"
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
	if cfg.DB.DSNEnvKey == "" {
		cfg.DB.DSNEnvKey = "GAME_DB_DSN"
	}
	if cfg.DB.MaxOpenConns <= 0 {
		cfg.DB.MaxOpenConns = 20
	}
	if cfg.DB.MaxIdleConns <= 0 {
		cfg.DB.MaxIdleConns = 10
	}
	if cfg.DB.ConnMaxLifetimeSeconds <= 0 {
		cfg.DB.ConnMaxLifetimeSeconds = 1800
	}
	if cfg.DB.ConnMaxIdleTimeSeconds <= 0 {
		cfg.DB.ConnMaxIdleTimeSeconds = 300
	}
	if cfg.State.OfflineTTLSec <= 0 {
		cfg.State.OfflineTTLSec = 120
	}
	if cfg.State.CleanupIntervalSec <= 0 {
		cfg.State.CleanupIntervalSec = 10
	}
	if cfg.State.OwnerCheckIntervalSec <= 0 {
		cfg.State.OwnerCheckIntervalSec = 5
	}
	if cfg.State.OwnerTTLSec <= cfg.State.OwnerCheckIntervalSec {
		cfg.State.OwnerTTLSec = 120
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "127.0.0.1:6379"
	}
	if cfg.Redis.PasswordEnvKey == "" {
		cfg.Redis.PasswordEnvKey = "GAME_REDIS_PASSWORD"
	}
	if cfg.Redis.NodeKeyPrefix == "" {
		cfg.Redis.NodeKeyPrefix = "game:gameserver"
	}
	if cfg.Redis.PlayerOwnerKeyPrefix == "" {
		cfg.Redis.PlayerOwnerKeyPrefix = "game:player_owner"
	}
	if cfg.Redis.NodeHeartbeatSec <= 0 {
		cfg.Redis.NodeHeartbeatSec = 5
	}
	if cfg.Redis.NodeTTLSec <= cfg.Redis.NodeHeartbeatSec {
		cfg.Redis.NodeTTLSec = cfg.Redis.NodeHeartbeatSec * 3
	}
	if cfg.WebSearch.BaseURL == "" {
		cfg.WebSearch.BaseURL = "https://zh.wikipedia.org/w/api.php"
	}
	if cfg.WebSearch.TimeoutMS <= 0 {
		cfg.WebSearch.TimeoutMS = 2000
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
