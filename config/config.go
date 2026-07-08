package config

import (
	"io/ioutil"
	"log"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构体
type Config struct {
	DB         DBConfig         `yaml:"db"`
	Cache      CacheConfig      `yaml:"cache"`
	Redis      RedisConfig      `yaml:"redis"`
	Server     ServerConfig     `yaml:"server"`
	WebSocket  WebSocketConfig  `yaml:"websocket"`
}

// DBConfig 数据库配置
type DBConfig struct {
	Path            string        `yaml:"path"`
	DebugMode       bool          `yaml:"debug_mode"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	CacheTTL        time.Duration `yaml:"cache_ttl"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	IdleThreshold   time.Duration `yaml:"idle_threshold"`
	LockShardCount  int           `yaml:"lock_shard_count"`
	DebugMode       bool          `yaml:"debug_mode"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	DB       int           `yaml:"db"`
	PoolSize int           `yaml:"pool_size"`
	MinIdle  int           `yaml:"min_idle"`
	Timeout  time.Duration `yaml:"timeout"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int `yaml:"port"`
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	Port           int           `yaml:"port"`           // 监听端口
	MaxConnections int           `yaml:"max_connections"` // 最大连接数
	ReadBufferSize int           `yaml:"read_buffer_size"` // 读取缓冲区大小
	WriteBufferSize int          `yaml:"write_buffer_size"` // 写入缓冲区大小
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // 心跳间隔
	WriteWait       time.Duration `yaml:"write_wait"`       // 写入超时
	PongWait        time.Duration `yaml:"pong_wait"`        // Pong等待时间
	NodeID          string        `yaml:"node_id"`          // 节点ID
	SessionExpire   int64         `yaml:"session_expire"`   // 会话过期时间（秒）
}

// GlobalConfig 全局配置实例
var GlobalConfig Config

// LoadConfig 从文件加载配置
func LoadConfig(filePath string) error {
	// 读取配置文件
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 解析YAML
	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return err
	}

	// 设置默认值
	setDefaults()

	log.Println("Config loaded successfully")
	return nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	// 数据库默认配置
	if GlobalConfig.DB.Path == "" {
		GlobalConfig.DB.Path = "default.db"
	}
	if GlobalConfig.DB.MaxOpenConns == 0 {
		GlobalConfig.DB.MaxOpenConns = 50
	}
	if GlobalConfig.DB.MaxIdleConns == 0 {
		GlobalConfig.DB.MaxIdleConns = 20
	}
	if GlobalConfig.DB.ConnMaxLifetime == 0 {
		GlobalConfig.DB.ConnMaxLifetime = time.Hour
	}
	if GlobalConfig.DB.ConnMaxIdleTime == 0 {
		GlobalConfig.DB.ConnMaxIdleTime = 30 * time.Minute
	}

	// 缓存默认配置
	if GlobalConfig.Cache.CacheTTL == 0 {
		GlobalConfig.Cache.CacheTTL = 5 * time.Minute
	}
	if GlobalConfig.Cache.CleanupInterval == 0 {
		GlobalConfig.Cache.CleanupInterval = 2 * time.Minute
	}
	if GlobalConfig.Cache.IdleThreshold == 0 {
		GlobalConfig.Cache.IdleThreshold = 10 * time.Minute
	}
	if GlobalConfig.Cache.LockShardCount == 0 {
		GlobalConfig.Cache.LockShardCount = 32
	}

	// Redis默认配置
	if GlobalConfig.Redis.Addr == "" {
		GlobalConfig.Redis.Addr = "localhost:6379"
	}
	if GlobalConfig.Redis.PoolSize == 0 {
		GlobalConfig.Redis.PoolSize = 10
	}
	if GlobalConfig.Redis.MinIdle == 0 {
		GlobalConfig.Redis.MinIdle = 5
	}
	if GlobalConfig.Redis.Timeout == 0 {
		GlobalConfig.Redis.Timeout = 5 * time.Second
	}

	// 服务器默认配置
	if GlobalConfig.Server.Port == 0 {
		GlobalConfig.Server.Port = 8080
	}

	// WebSocket默认配置
	if GlobalConfig.WebSocket.Port == 0 {
		GlobalConfig.WebSocket.Port = 8081
	}
	if GlobalConfig.WebSocket.MaxConnections == 0 {
		GlobalConfig.WebSocket.MaxConnections = 10000
	}
	if GlobalConfig.WebSocket.ReadBufferSize == 0 {
		GlobalConfig.WebSocket.ReadBufferSize = 4096
	}
	if GlobalConfig.WebSocket.WriteBufferSize == 0 {
		GlobalConfig.WebSocket.WriteBufferSize = 4096
	}
	if GlobalConfig.WebSocket.HeartbeatInterval == 0 {
		GlobalConfig.WebSocket.HeartbeatInterval = 30 * time.Second
	}
	if GlobalConfig.WebSocket.WriteWait == 0 {
		GlobalConfig.WebSocket.WriteWait = 10 * time.Second
	}
	if GlobalConfig.WebSocket.PongWait == 0 {
		GlobalConfig.WebSocket.PongWait = 60 * time.Second
	}
	if GlobalConfig.WebSocket.NodeID == "" {
		GlobalConfig.WebSocket.NodeID = "node_1"
	}
	if GlobalConfig.WebSocket.SessionExpire == 0 {
		GlobalConfig.WebSocket.SessionExpire = 86400 // 24小时
	}
}

// GetConfig 获取全局配置
func GetConfig() Config {
	return GlobalConfig
}
