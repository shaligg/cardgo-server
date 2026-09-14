// Package loginserver 负责独立登录进程的配置、装配和生命周期。
package loginserver

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/loginserver.local.yaml"

// Config 保存账号数据库、登录 HTTP、账号会话和票据签发配置。
type Config struct {
	HTTPSecurity HTTPSecurityConfig `yaml:"http_security"`
	Account      struct {
		SessionIdleTTLSec int64 `yaml:"session_idle_ttl_sec"`
	} `yaml:"account"`
	DB struct {
		DSNEnvKey          string `yaml:"dsn_env_key"`
		MaxOpenConns       int    `yaml:"max_open_conns"`
		MaxIdleConns       int    `yaml:"max_idle_conns"`
		ConnMaxLifetimeSec int    `yaml:"conn_max_lifetime_sec"`
		ConnMaxIdleTimeSec int    `yaml:"conn_max_idle_time_sec"`
	} `yaml:"db"`

	HTTP struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"http"`
	Auth struct {
		Issuer       string `yaml:"issuer"`
		Algorithm    string `yaml:"algorithm"`
		TicketTTLSec int    `yaml:"ticket_ttl_sec"`
		SecretEnvKey string `yaml:"secret_env_key"`
	} `yaml:"auth"`
	Redis struct {
		Addr                 string `yaml:"addr"`
		PasswordEnvKey       string `yaml:"password_env_key"`
		DB                   int    `yaml:"db"`
		NodeKeyPrefix        string `yaml:"node_key_prefix"`
		PlayerOwnerKeyPrefix string `yaml:"player_owner_key_prefix"`
	} `yaml:"redis"`
}

// LoadConfig 严格读取登录配置，关键配置缺失或显式无效时直接失败。
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = defaultConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read loginserver config: %w", err)
	}
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode loginserver config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func LoadConfigFromEnv() (Config, error) {
	return LoadConfig(os.Getenv("GAME_CONFIG"))
}

func (cfg Config) validate() error {
	if cfg.Account.SessionIdleTTLSec <= 0 || cfg.Account.SessionIdleTTLSec > 31536000 {
		return fmt.Errorf("invalid account idle ttl (must be within one year)")
	}
	if strings.TrimSpace(cfg.DB.DSNEnvKey) == "" || cfg.DB.MaxOpenConns <= 0 || cfg.DB.MaxIdleConns < 0 || cfg.DB.MaxIdleConns > cfg.DB.MaxOpenConns || cfg.DB.ConnMaxLifetimeSec <= 0 || cfg.DB.ConnMaxLifetimeSec > 86400 || cfg.DB.ConnMaxIdleTimeSec <= 0 || cfg.DB.ConnMaxIdleTimeSec > cfg.DB.ConnMaxLifetimeSec {
		return fmt.Errorf("invalid account database configuration")
	}
	if cfg.HTTPSecurity.RequestsPerMinute <= 0 || cfg.HTTPSecurity.RequestsPerMinute > 1000000 || cfg.HTTPSecurity.MaxBodyBytes < 128 || cfg.HTTPSecurity.MaxBodyBytes > 1048576 {
		return fmt.Errorf("invalid http security configuration")
	}
	for _, cidr := range cfg.HTTPSecurity.TrustedProxies {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("invalid trusted proxy CIDR")
		}
	}

	if strings.TrimSpace(cfg.HTTP.Host) == "" || cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("invalid loginserver http host or port")
	}
	if cfg.Auth.Algorithm != "hmac-sha256" {
		return fmt.Errorf("unsupported auth algorithm: %s", cfg.Auth.Algorithm)
	}
	if strings.TrimSpace(cfg.Auth.Issuer) == "" || strings.TrimSpace(cfg.Auth.SecretEnvKey) == "" {
		return fmt.Errorf("auth issuer and secret_env_key are required")
	}
	// time.Duration 以纳秒保存 TTL，拒绝导致时长溢出的配置。
	if cfg.Auth.TicketTTLSec <= 0 || int64(cfg.Auth.TicketTTLSec) > (1<<63-1)/1_000_000_000 {
		return fmt.Errorf("invalid auth ticket_ttl_sec: %d", cfg.Auth.TicketTTLSec)
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" || cfg.Redis.DB < 0 ||
		strings.TrimSpace(cfg.Redis.NodeKeyPrefix) == "" || strings.TrimSpace(cfg.Redis.PlayerOwnerKeyPrefix) == "" {
		return fmt.Errorf("invalid redis address, db or key prefixes")
	}
	return nil
}
