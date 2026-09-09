package repo

import (
	"context"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/repo/model"
)

// CreateChatMessage 持久化一条聊天消息。
func (r *DBPlayerRepository) CreateChatMessage(ctx context.Context, message ChatMessageRecord) (ChatMessageRecord, error) {
	row := model.ChatMessage{
		MsgID:     message.MsgID,
		ChannelID: message.ChannelID,
		UID:       message.UID,
		Content:   message.Content,
		ReqID:     message.ReqID,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return ChatMessageRecord{}, fmt.Errorf("create chat message: %w", err)
	}
	return toChatMessageRecord(row), nil
}

// ListChatMessages 从最新消息向更早消息分页，单页结果按时间正序返回。
func (r *DBPlayerRepository) ListChatMessages(ctx context.Context, channelID string, beforeID uint64, limit int) ([]ChatMessageRecord, uint64, error) {
	var rows []model.ChatMessage
	query := r.db.WithContext(ctx).Where("channel_id = ?", channelID)
	if beforeID > 0 {
		query = query.Where("id < ?", beforeID)
	}
	if err := query.Order("id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list chat messages: %w", err)
	}
	nextCursor := uint64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		nextCursor = rows[len(rows)-1].ID
	}

	result := make([]ChatMessageRecord, len(rows))
	for i, row := range rows {
		result[len(rows)-1-i] = toChatMessageRecord(row)
	}
	return result, nextCursor, nil
}

func toChatMessageRecord(row model.ChatMessage) ChatMessageRecord {
	return ChatMessageRecord{
		MsgID:     row.MsgID,
		ChannelID: row.ChannelID,
		UID:       row.UID,
		Content:   row.Content,
		ReqID:     row.ReqID,
		CreatedAt: row.CreatedAt.Unix(),
	}
}
