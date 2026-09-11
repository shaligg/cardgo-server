// Package loginserver 负责独立登录进程的配置、装配和生命周期。
package loginserver

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = "configs/loginserver.local.yaml"

// Config 仅保存登录 HTTP、票据签发和共享 Redis 的配置。
type Config struct {
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
