// Package gameserver 负责 GameServer 进程的配置、装配和生命周期。
package gameserver

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/gameserver.local.yaml"

// Config 保存 GameServer 启动和运行所需的全部配置。
type Config struct {
	Server struct {
		NodeID           string `yaml:"node_id"`
		AdminHost        string `yaml:"admin_host"`
		AdminPort        int    `yaml:"admin_port"`
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

// LoadConfig 严格读取 GameServer 配置，未知字段或关键配置无效时直接失败。
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode gameserver config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("validate gameserver config: %w", err)
	}
	return cfg, nil
}

func LoadConfigFromEnv() (Config, error) {
	return LoadConfig(os.Getenv("GAME_CONFIG"))
}

func (cfg Config) validate() error {
	if strings.TrimSpace(cfg.Server.NodeID) == "" {
		return fmt.Errorf("server.node_id is required")
	}
	if strings.TrimSpace(cfg.Server.AdminHost) == "" || cfg.Server.AdminPort < 1 || cfg.Server.AdminPort > 65535 {
		return fmt.Errorf("invalid server.admin_host or server.admin_port")
	}
	if strings.TrimSpace(cfg.Server.WSHost) == "" || cfg.Server.WSPort < 1 || cfg.Server.WSPort > 65535 {
		return fmt.Errorf("invalid server.ws_host or server.ws_port")
	}
	advertisedWS, err := url.Parse(cfg.Server.AdvertisedWSAddr)
	if err != nil || (advertisedWS.Scheme != "ws" && advertisedWS.Scheme != "wss") || advertisedWS.Host == "" {
		return fmt.Errorf("invalid server.advertised_ws_addr")
	}
	if cfg.Server.MaxConnections <= 0 || cfg.Server.DispatcherShards <= 0 {
		return fmt.Errorf("server.max_connections and server.dispatcher_shards must be positive")
	}
	if cfg.Auth.Algorithm != "hmac-sha256" {
		return fmt.Errorf("unsupported auth.algorithm: %s", cfg.Auth.Algorithm)
	}
	if strings.TrimSpace(cfg.Auth.Issuer) == "" || strings.TrimSpace(cfg.Auth.SecretEnvKey) == "" || cfg.Auth.NonceTTLSec <= 0 {
		return fmt.Errorf("invalid auth issuer, secret_env_key or nonce_ttl_sec")
	}
	if cfg.Admin.RequireAuth && strings.TrimSpace(cfg.Admin.TokenEnvKey) == "" {
		return fmt.Errorf("admin.token_env_key is required when admin authentication is enabled")
	}
	if cfg.WS.HeartbeatIntervalSec <= 0 || cfg.WS.PongWaitSec <= cfg.WS.HeartbeatIntervalSec || cfg.WS.WriteWaitSec <= 0 {
		return fmt.Errorf("invalid ws heartbeat, pong wait or write wait")
	}
	if cfg.WS.SendQueueSize <= 0 || cfg.WS.BizMinGapMS < 0 || cfg.WS.MaxMessageBytes <= 0 {
		return fmt.Errorf("invalid ws send queue, business interval or message size")
	}
	if strings.TrimSpace(cfg.DB.DSNEnvKey) == "" || cfg.DB.MaxOpenConns <= 0 || cfg.DB.MaxIdleConns <= 0 || cfg.DB.MaxIdleConns > cfg.DB.MaxOpenConns {
		return fmt.Errorf("invalid db environment key or connection pool size")
	}
	if cfg.DB.ConnMaxLifetimeSeconds <= 0 || cfg.DB.ConnMaxIdleTimeSeconds <= 0 {
		return fmt.Errorf("invalid db connection lifetime or idle time")
	}
	if cfg.State.OfflineTTLSec <= 0 || cfg.State.OwnerCheckIntervalSec <= 0 || cfg.State.OwnerTTLSec <= cfg.State.OwnerCheckIntervalSec {
		return fmt.Errorf("invalid state offline ttl, owner check interval or owner ttl")
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" || cfg.Redis.DB < 0 ||
		strings.TrimSpace(cfg.Redis.NodeKeyPrefix) == "" || strings.TrimSpace(cfg.Redis.PlayerOwnerKeyPrefix) == "" {
		return fmt.Errorf("invalid redis address, db or key prefixes")
	}
	if cfg.Redis.NodeHeartbeatSec <= 0 || cfg.Redis.NodeTTLSec <= cfg.Redis.NodeHeartbeatSec {
		return fmt.Errorf("redis.node_ttl_sec must be greater than redis.node_heartbeat_sec")
	}
	webSearchURL, err := url.Parse(cfg.WebSearch.BaseURL)
	if err != nil || (webSearchURL.Scheme != "http" && webSearchURL.Scheme != "https") || webSearchURL.Host == "" || cfg.WebSearch.TimeoutMS <= 0 {
		return fmt.Errorf("invalid web_search.base_url or web_search.timeout_ms")
	}
	if strings.TrimSpace(cfg.GameData.ItemConfigPath) == "" || strings.TrimSpace(cfg.GameData.CardConfigPath) == "" ||
		strings.TrimSpace(cfg.GameData.OrderConfigPath) == "" || strings.TrimSpace(cfg.GameData.LevelConfigPath) == "" ||
		strings.TrimSpace(cfg.GameData.FacilityConfigPath) == "" {
		return fmt.Errorf("all gamedata config paths are required")
	}
	return nil
}
