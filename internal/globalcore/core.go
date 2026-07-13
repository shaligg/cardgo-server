package globalcore

// Core 聚合当前 GameServer 可使用的公共领域核心接口。
//
// globalcore 不是独立公共服进程，也不是单纯 remote client；它承载公共领域接口、
// DTO 和可复用规则。MVP 阶段可以同进程本地实现，未来可把部分实现替换为远程客户端。
type Core struct {
	Guild GuildService
	Chat  ChatService
	Rank  RankService
	World WorldService
}
