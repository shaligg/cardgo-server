package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// FriendApply 解析好友申请并交给好友领域服务。
func (h *BizHandler) FriendApply(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.FriendApplyRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid friend_apply payload"}
	}
	if err := h.FriendService.Apply(ctx, targetUID, req.TargetUID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"target_uid": req.TargetUID, "status": "outgoing"}, nil
}

// FriendApprove 同意目标玩家发来的好友申请。
func (h *BizHandler) FriendApprove(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.FriendApproveRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid friend_approve payload"}
	}
	if err := h.FriendService.Approve(ctx, targetUID, req.TargetUID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"target_uid": req.TargetUID, "status": "accepted"}, nil
}

// FriendRemove 删除好友关系或撤销尚未处理的申请。
func (h *BizHandler) FriendRemove(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.FriendRemoveRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid friend_remove payload"}
	}
	if err := h.FriendService.Remove(ctx, targetUID, req.TargetUID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"target_uid": req.TargetUID, "status": "removed"}, nil
}

// FriendList 返回好友关系和待处理申请。
func (h *BizHandler) FriendList(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.FriendListRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid friend_list payload"}
	}
	items, nextCursor, err := h.FriendService.List(ctx, targetUID, req.Cursor, req.Limit)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"friends": items, "next_cursor": nextCursor}, nil
}

// GuildCreate 创建公会并把当前玩家设为会长。
func (h *BizHandler) GuildCreate(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildCreateRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_create payload"}
	}
	guild, err := h.GuildService.Create(ctx, targetUID, req.Name, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"guild": guild}, nil
}

// GuildSearch 按名称分页搜索公会。
func (h *BizHandler) GuildSearch(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildSearchRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_search payload"}
	}
	guilds, nextCursor, err := h.GuildService.Search(ctx, targetUID, req.Keyword, req.Cursor, req.Limit)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"guilds": guilds, "next_cursor": nextCursor}, nil
}

// GuildApplyJoin 提交入会申请。
func (h *BizHandler) GuildApplyJoin(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildApplyJoinRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_apply_join payload"}
	}
	if err := h.GuildService.ApplyJoin(ctx, targetUID, req.GuildID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"guild_id": req.GuildID, "status": "applied"}, nil
}

// GuildApproveJoin 由会长同意目标玩家加入公会。
func (h *BizHandler) GuildApproveJoin(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildApproveJoinRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_approve_join payload"}
	}
	if err := h.GuildService.ApproveJoin(ctx, targetUID, req.GuildID, req.TargetUID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"guild_id": req.GuildID, "target_uid": req.TargetUID, "status": "joined"}, nil
}

// GuildLeave 退出当前公会；会长退出时由公会服务处理转让或解散。
func (h *BizHandler) GuildLeave(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildLeaveRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_leave payload"}
	}
	if err := h.GuildService.Leave(ctx, targetUID, req.ReqID); err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"status": "left"}, nil
}

// GuildGet 查询指定公会，guild_id 为空时查询当前玩家所属公会。
func (h *BizHandler) GuildGet(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildGetRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_get payload"}
	}
	guild, err := h.GuildService.Get(ctx, targetUID, req.GuildID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"guild": guild}, nil
}

// GuildListApplications 返回当前会长可审批的入会申请。
func (h *BizHandler) GuildListApplications(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GuildListApplicationsRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid guild_list_applications payload"}
	}
	applications, nextCursor, err := h.GuildService.ListApplications(ctx, targetUID, req.GuildID, req.Cursor, req.Limit)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"applications": applications, "next_cursor": nextCursor}, nil
}

// ChatSend 向世界频道或当前玩家的公会频道发送消息。
func (h *BizHandler) ChatSend(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ChatSendRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid chat_send payload"}
	}
	message, err := h.ChatService.SendChannelMsg(ctx, req.Channel, targetUID, req.Content, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"message": message}, nil
}

// ChatHistory 分页拉取世界频道或当前玩家的公会频道历史消息。
func (h *BizHandler) ChatHistory(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ChatHistoryRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid chat_history payload"}
	}
	messages, nextCursor, err := h.ChatService.PullHistory(ctx, req.Channel, targetUID, req.Cursor, req.Limit)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{"messages": messages, "next_cursor": nextCursor}, nil
}
