package globalcore

import "context"

// ChatMessage 是聊天公共领域对外返回的消息 DTO。
type ChatMessage struct {
	MsgID     string
	ChannelID string
	UID       string
	Content   string
	TS        int64
}

// ChatService 定义聊天公共领域能力。
//
// 实现可以是同进程 LocalService，也可以是未来的 RemoteClient；
// 接口参数必须保持 DTO 化，不能依赖 WebSocket 连接、session 或在线内存。
type ChatService interface {
	SendChannelMsg(ctx context.Context, channelID string, uid string, content string, reqID string) error
	PullHistory(ctx context.Context, channelID string, cursor string, limit int) ([]ChatMessage, string, error)
}
