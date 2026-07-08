package redis

import (
	"context"
	"log"
	"time"

	"github.com/bigfish/go_orm_1/config"
	"github.com/redis/go-redis/v9"
)

// Client Redis客户端实例
var Client *redis.Client

// Init 初始化Redis客户端
func Init() error {
	// 获取Redis配置
	redisConfig := config.GlobalConfig.Redis

	// 创建Redis客户端
	Client = redis.NewClient(&redis.Options{
		Addr:         redisConfig.Addr,
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		PoolSize:     redisConfig.PoolSize,
		MinIdleConns: redisConfig.MinIdle,
		DialTimeout:  redisConfig.Timeout,
		ReadTimeout:  redisConfig.Timeout,
		WriteTimeout: redisConfig.Timeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), redisConfig.Timeout)
	defer cancel()

	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return err
	}

	log.Println("Redis client initialized successfully")
	return nil
}

// Get 获取key对应的值
func Get(ctx context.Context, key string) (string, error) {
	return Client.Get(ctx, key).Result()
}

// Set 设置key-value，支持过期时间
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return Client.Set(ctx, key, value, expiration).Err()
}

// HGet 获取哈希表中的字段值
func HGet(ctx context.Context, key, field string) (string, error) {
	return Client.HGet(ctx, key, field).Result()
}

// HSet 设置哈希表中的字段值
func HSet(ctx context.Context, key, field string, value interface{}) error {
	return Client.HSet(ctx, key, field, value).Err()
}

// HGetAll 获取哈希表中的所有字段和值
func HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return Client.HGetAll(ctx, key).Result()
}

// ZAdd 添加元素到有序集合
func ZAdd(ctx context.Context, key string, score float64, member string) error {
	return Client.ZAdd(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRangeWithScores 获取有序集合指定范围的元素和分数（升序）
func ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return Client.ZRangeWithScores(ctx, key, start, stop).Result()
}

// ZRevRangeWithScores 获取有序集合指定范围的元素和分数（降序）
func ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return Client.ZRevRangeWithScores(ctx, key, start, stop).Result()
}

// ZRank 获取元素在有序集合中的排名（升序，从0开始）
func ZRank(ctx context.Context, key, member string) (int64, error) {
	return Client.ZRank(ctx, key, member).Result()
}

// ZRevRank 获取元素在有序集合中的排名（降序，从0开始）
func ZRevRank(ctx context.Context, key, member string) (int64, error) {
	return Client.ZRevRank(ctx, key, member).Result()
}

// ZScore 获取元素在有序集合中的分数
func ZScore(ctx context.Context, key, member string) (float64, error) {
	return Client.ZScore(ctx, key, member).Result()
}

// Del 删除指定的key
func Del(ctx context.Context, keys ...string) error {
	return Client.Del(ctx, keys...).Err()
}

// Expire 设置key的过期时间
func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return Client.Expire(ctx, key, expiration).Err()
}

// Scan 扫描匹配的key
func Scan(ctx context.Context, cursor uint64, match string, count int64) (uint64, []string, error) {
	keys, nextCursor, err := Client.Scan(ctx, cursor, match, count).Result()
	return nextCursor, keys, err
}

// // FlushDB 清空当前数据库
// func FlushDB(ctx context.Context) error {
// 	return Client.FlushDB(ctx).Err()
// }

// GetClient 获取Redis客户端实例
func GetClient() *redis.Client {
	return Client
}
