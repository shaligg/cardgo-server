package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bigfish/go_orm_1/config"
	"github.com/bigfish/go_orm_1/redis"
)

func main() {
	// 加载配置文件
	err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化Redis
	err = redis.Init()
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}

	ctx := context.Background()

	// 测试1: 基本的get/set操作
	log.Println("\n=== Testing Basic get/set ===")
	err = redis.Set(ctx, "test_key", "test_value", 10*time.Second)
	if err != nil {
		log.Printf("Set failed: %v", err)
	} else {
		log.Println("Set key: test_key, value: test_value")
	}

	value, err := redis.Get(ctx, "test_key")
	if err != nil {
		log.Printf("Get failed: %v", err)
	} else {
		log.Printf("Get key: test_key, value: %s", value)
	}

	// 测试2: 哈希操作hget/hset
	log.Println("\n=== Testing Hash hget/hset ===")
	err = redis.HSet(ctx, "test_hash", "field1", "value1")
	if err != nil {
		log.Printf("HSet failed: %v", err)
	} else {
		log.Println("HSet hash: test_hash, field: field1, value: value1")
	}

	err = redis.HSet(ctx, "test_hash", "field2", "value2")
	if err != nil {
		log.Printf("HSet failed: %v", err)
	} else {
		log.Println("HSet hash: test_hash, field: field2, value: value2")
	}

	hvalue1, err := redis.HGet(ctx, "test_hash", "field1")
	if err != nil {
		log.Printf("HGet failed: %v", err)
	} else {
		log.Printf("HGet hash: test_hash, field: field1, value: %s", hvalue1)
	}

	hvalue2, err := redis.HGet(ctx, "test_hash", "field2")
	if err != nil {
		log.Printf("HGet failed: %v", err)
	} else {
		log.Printf("HGet hash: test_hash, field: field2, value: %s", hvalue2)
	}

	hall, err := redis.HGetAll(ctx, "test_hash")
	if err != nil {
		log.Printf("HGetAll failed: %v", err)
	} else {
		log.Printf("HGetAll hash: test_hash, values: %v", hall)
	}

	// 测试3: 集合排行和排名读取
	log.Println("\n=== Testing Sorted Set ===")
	// 添加测试数据
	users := map[string]float64{
		"user1": 100,
		"user2": 200,
		"user3": 150,
		"user4": 300,
		"user5": 250,
	}

	for user, score := range users {
		err = redis.ZAdd(ctx, "leaderboard", score, user)
		if err != nil {
			log.Printf("ZAdd failed for %s: %v", user, err)
		} else {
			log.Printf("ZAdd leaderboard: user: %s, score: %.2f", user, score)
		}
	}

	// 获取升序排名（从0开始）
	log.Println("\n=== Ascending Rank ===")
	rank, err := redis.ZRank(ctx, "leaderboard", "user3")
	if err != nil {
		log.Printf("ZRank failed: %v", err)
	} else {
		log.Printf("User3 rank (ascending): %d", rank)
	}

	// 获取降序排名（从0开始）
	log.Println("\n=== Descending Rank ===")
	revRank, err := redis.ZRevRank(ctx, "leaderboard", "user3")
	if err != nil {
		log.Printf("ZRevRank failed: %v", err)
	} else {
		log.Printf("User3 rank (descending): %d", revRank)
	}

	// 获取分数
	log.Println("\n=== Score ===")
	score, err := redis.ZScore(ctx, "leaderboard", "user3")
	if err != nil {
		log.Printf("ZScore failed: %v", err)
	} else {
		log.Printf("User3 score: %.2f", score)
	}

	// 获取前3名（降序）
	log.Println("\n=== Top 3 Users ===")
	topUsers, err := redis.ZRevRangeWithScores(ctx, "leaderboard", 0, 2)
	if err != nil {
		log.Printf("ZRevRangeWithScores failed: %v", err)
	} else {
		for i, user := range topUsers {
			log.Printf("Rank %d: %s, Score: %.2f", i+1, user.Member, user.Score)
		}
	}

	// 测试4: 清理key
	log.Println("\n=== Testing Key Cleanup ===")
	err = redis.Del(ctx, "test_key", "test_hash")
	if err != nil {
		log.Printf("Del failed: %v", err)
	} else {
		log.Println("Deleted keys: test_key, test_hash")
	}

	// 验证key是否被删除
	value, err = redis.Get(ctx, "test_key")
	if err != nil {
		log.Printf("Get test_key after delete: %v (expected error)", err)
	}

	// 测试5: Scan操作
	log.Println("\n=== Testing Scan ===")
	var cursor uint64
	var keys []string

	// 先添加一些测试key
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("scan_test_%d", i)
		err = redis.Set(ctx, key, fmt.Sprintf("value_%d", i), 30*time.Second)
		if err != nil {
			log.Printf("Set %s failed: %v", key, err)
		}
	}

	// 执行scan
	for {
		var err error
		cursor, keys, err = redis.Scan(ctx, cursor, "scan_test_*", 2)
		if err != nil {
			log.Printf("Scan failed: %v", err)
			break
		}

		log.Printf("Scan cursor: %d, found keys: %v", cursor, keys)

		if cursor == 0 {
			break
		}
	}

	// 清理测试key
	err = redis.Del(ctx, "leaderboard")
	if err != nil {
		log.Printf("Del leaderboard failed: %v", err)
	}

	// 清理scan测试key
	cursor = 0
	for {
		var err error
		cursor, keys, err = redis.Scan(ctx, cursor, "scan_test_*", 10)
		if err != nil {
			log.Printf("Scan for cleanup failed: %v", err)
			break
		}

		if len(keys) > 0 {
			err = redis.Del(ctx, keys...)
			if err != nil {
				log.Printf("Del scan keys failed: %v", err)
			}
		}

		if cursor == 0 {
			break
		}
	}

	log.Println("\n=== All tests completed ===")
}
