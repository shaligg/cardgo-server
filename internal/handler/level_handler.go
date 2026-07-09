package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
)

// levelHandler 处理关卡与局内战斗协议。
//
// 当前 MVP 的关卡运行时仍在 BattleService 内存中，后续房间化时优先替换 BattleService 边界。
type levelHandler struct {
	battleService *battlesvc.Service
}

func newLevelHandler(battleService *battlesvc.Service) *levelHandler {
	return &levelHandler{battleService: battleService}
}

func (h *levelHandler) Start(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelStartRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_start payload"}
	}
	if h.battleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	session, err := h.battleService.StartLevel(ctx, targetUID, req.LevelID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"session": session,
	}, nil
}

func (h *levelHandler) PlayCard(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelPlayCardRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_play_card payload"}
	}
	sessionID := req.LevelSessionID
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if h.battleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	result, err := h.battleService.PlayCard(ctx, targetUID, sessionID, req.CardID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"result": result,
	}, nil
}

func (h *levelHandler) Settle(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelSettleRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_settle payload"}
	}
	sessionID := req.LevelSessionID
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if h.battleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	result, err := h.battleService.SettleLevel(ctx, targetUID, sessionID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"result": result,
	}, nil
}
