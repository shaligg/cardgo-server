package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	playersvc "github.com/bigfish/go_orm_1/internal/game/player"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// playerHandler 处理玩家基础协议。
//
// 它只持有玩家协议需要的依赖，避免总路由器随着玩法增加持续膨胀。
type playerHandler struct {
	playerService playersvc.Service
	online        *state.OnlineState
}

func newPlayerHandler(playerService playersvc.Service, online *state.OnlineState) *playerHandler {
	return &playerHandler{playerService: playerService, online: online}
}

func (h *playerHandler) GetProfile(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid get_profile payload"}
	}
	p, err := h.playerService.QueryProfile(ctx, targetUID)
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	syncOnlinePlayerState(h.online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}

func (h *playerHandler) AddGold(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.AddGoldRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid add_gold payload"}
	}
	p, err := h.playerService.AddGold(ctx, targetUID, req.Delta, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncOnlinePlayerState(h.online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}

func (h *playerHandler) ConsumeGold(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ConsumeGoldRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid consume_gold payload"}
	}
	p, err := h.playerService.ConsumeGold(ctx, targetUID, req.Amount, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncOnlinePlayerState(h.online, p)
	return map[string]interface{}{
		"player": p,
	}, nil
}
