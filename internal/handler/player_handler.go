package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// PlayerGetProfile 处理玩家基础资料查询协议。
func (h *BizHandler) PlayerGetProfile(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid get_profile payload"}
	}
	p, err := h.PlayerService.QueryProfile(ctx, targetUID)
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	syncOnlinePlayerState(h.Online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}

// PlayerAddGold 处理调试环境增加金币协议。
func (h *BizHandler) PlayerAddGold(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.AddGoldRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid add_gold payload"}
	}
	p, err := h.PlayerService.AddGold(ctx, targetUID, req.Delta, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncOnlinePlayerState(h.Online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}

// PlayerConsumeGold 处理调试环境消耗金币协议。
func (h *BizHandler) PlayerConsumeGold(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ConsumeGoldRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid consume_gold payload"}
	}
	p, err := h.PlayerService.ConsumeGold(ctx, targetUID, req.Amount, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncOnlinePlayerState(h.Online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}
