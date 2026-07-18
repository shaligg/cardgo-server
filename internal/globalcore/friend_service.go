package globalcore

import "context"

// FriendItem 是好友列表返回的轻量 DTO。
type FriendItem struct {
	UID      string
	Level    int
	Nickname string
	Status   string
}

// FriendService 定义好友公共领域能力。
//
// 好友关系属于跨玩家公共数据，接口必须保持 DTO 化，不能依赖 GameServer 连接、
// session 或在线热状态；未来可由 LocalService 替换为 RemoteClient。
type FriendService interface {
	Apply(ctx context.Context, uid string, targetUID string, reqID string) error
	Approve(ctx context.Context, uid string, targetUID string, reqID string) error
	Remove(ctx context.Context, uid string, targetUID string, reqID string) error
	List(ctx context.Context, uid string, cursor string, limit int) ([]FriendItem, string, error)
}
