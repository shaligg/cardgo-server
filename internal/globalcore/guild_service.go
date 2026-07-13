package globalcore

import "context"

// GuildService 定义公会公共领域能力。
//
// 公会数据以 DB/Redis 等共享存储为权威，不能依赖某个 GameServer 的连接对象或在线热状态。
type GuildService interface {
	ApplyJoin(ctx context.Context, uid string, guildID string, reqID string) error
	ApproveJoin(ctx context.Context, operatorUID string, guildID string, targetUID string, reqID string) error
}
