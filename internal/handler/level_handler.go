package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// LevelStart 处理关卡开始协议。
func (h *BizHandler) LevelStart(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelStartRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_start payload"}
	}
	if h.BattleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	session, err := h.BattleService.StartLevel(ctx, targetUID, req.LevelID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"session": session,
	}, nil
}

// LevelPlayCard 处理关卡出牌协议。
func (h *BizHandler) LevelPlayCard(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelPlayCardRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_play_card payload"}
	}
	sessionID := req.LevelSessionID
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if h.BattleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	result, err := h.BattleService.PlayCard(ctx, targetUID, sessionID, req.CardID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"result": result,
	}, nil
}

// LevelSettle 处理关卡结算协议。
func (h *BizHandler) LevelSettle(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.LevelSettleRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid level_settle payload"}
	}
	sessionID := req.LevelSessionID
	if sessionID == "" {
		sessionID = req.SessionID
	}
	if h.BattleService == nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: "battle service is nil"}
	}
	result, err := h.BattleService.SettleLevel(ctx, targetUID, sessionID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.Online, *result.Player)
	}
	return map[string]interface{}{
		"result": result,
	}, nil
}
