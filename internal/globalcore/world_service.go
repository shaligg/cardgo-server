package globalcore

import "context"

// WorldService 定义世界级公共能力，例如公告、世界事件等。
//
// 世界级能力可能被多个 GameServer 或未来 GlobalServer 复用，参数必须显式且可序列化。
type WorldService interface {
	PublishAnnouncement(ctx context.Context, operatorUID string, content string, reqID string) error
}
