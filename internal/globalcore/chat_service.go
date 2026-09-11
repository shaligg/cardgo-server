package globalcore

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
)

// ChatMessage 是聊天公共领域对外返回的消息 DTO。
type ChatMessage struct {
	MsgID     string `json:"msg_id"`
	ChannelID string `json:"channel_id"`
	UID       string `json:"uid"`
	Content   string `json:"content"`
	TS        int64  `json:"ts"`
}

// ChatService 定义聊天公共领域能力。
//
// 实现可以是同进程 LocalService，也可以是未来的 RemoteClient；
// 接口参数必须保持 DTO 化，不能依赖 WebSocket 连接、session 或在线内存。
type ChatService interface {
	SendChannelMsg(ctx context.Context, channel string, uid string, content string, reqID string) (ChatMessage, error)
	PullHistory(ctx context.Context, channel string, uid string, cursor string, limit int) ([]ChatMessage, string, error)
}

// LocalChatService 是聊天领域的同进程实现。
type LocalChatService struct {
	Messages   repo.ChatRepository
	Membership repo.GuildMembershipReader
}

// SendChannelMsg 校验逻辑频道和内容后持久化消息。
func (s LocalChatService) SendChannelMsg(ctx context.Context, channel string, uid string, content string, reqID string) (ChatMessage, error) {
	if err := requireReqID(reqID); err != nil {
		return ChatMessage{}, err
	}
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 200 {
		return ChatMessage{}, ErrInvalidChatContent
	}
	channelID, err := s.resolveChannel(ctx, channel, uid)
	if err != nil {
		return ChatMessage{}, err
	}
	record, err := s.Messages.CreateChatMessage(ctx, repo.ChatMessageRecord{
		MsgID:     uuid.NewString(),
		ChannelID: channelID,
		UID:       uid,
		Content:   content,
		ReqID:     reqID,
	})
	if err != nil {
		return ChatMessage{}, err
	}
	return toChatMessage(record), nil
}

// PullHistory 从最新页开始向更早消息分页。
func (s LocalChatService) PullHistory(ctx context.Context, channel string, uid string, cursor string, limit int) ([]ChatMessage, string, error) {
	beforeID, pageSize, err := parsePage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	channelID, err := s.resolveChannel(ctx, channel, uid)
	if err != nil {
		return nil, "", err
	}
	rows, nextCursor, err := s.Messages.ListChatMessages(ctx, channelID, beforeID, pageSize)
	if err != nil {
		return nil, "", err
	}
	messages := make([]ChatMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, toChatMessage(row))
	}
	return messages, formatCursor(nextCursor), nil
}

func (s LocalChatService) resolveChannel(ctx context.Context, channel string, uid string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(channel)) {
	case "world":
		return "world", nil
	case "guild":
		guildID, err := s.Membership.GetGuildIDByUID(ctx, uid)
		if err != nil {
			return "", err
		}
		return guildChatChannelID(guildID), nil
	default:
		return "", ErrInvalidChatChannel
	}
}

// guildChatChannelID 统一生成公会聊天使用的持久化频道 ID。
func guildChatChannelID(guildID string) string {
	return "guild:" + guildID
}

func toChatMessage(record repo.ChatMessageRecord) ChatMessage {
	return ChatMessage{
		MsgID:     record.MsgID,
		ChannelID: record.ChannelID,
		UID:       record.UID,
		Content:   record.Content,
		TS:        record.CreatedAt,
	}
}
