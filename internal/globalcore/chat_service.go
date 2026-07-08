package globalcore

import "context"

type ChatMessage struct {
	MsgID     string
	ChannelID string
	UID       string
	Content   string
	TS        int64
}

type ChatService interface {
	SendChannelMsg(ctx context.Context, channelID string, uid string, content string, reqID string) error
	PullHistory(ctx context.Context, channelID string, cursor string, limit int) ([]ChatMessage, string, error)
}
