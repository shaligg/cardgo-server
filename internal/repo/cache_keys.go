package repo

import "fmt"

// playerCacheKey 生成玩家基础资料缓存 key。
//
// 该 key 带有玩家业务语义，放在 repo 层而不是 infra/cache，避免通用缓存模块感知业务对象。
func playerCacheKey(uid string) string {
	return fmt.Sprintf("player:%s", uid)
}
