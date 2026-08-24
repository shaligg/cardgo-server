package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// CardGetCards 处理玩家卡牌列表查询协议。
func (h *BizHandler) CardGetCards(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_get_cards payload"}
	}
	result, err := h.CardService.GetCards(ctx, targetUID)
	if err != nil {
		return nil, toBizError(err)
	}
	return result, nil
}

// CardSaveDeck 处理卡组保存协议。
func (h *BizHandler) CardSaveDeck(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.CardSaveDeckRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_save_deck payload"}
	}
	deck, err := h.CardService.SaveDeck(ctx, targetUID, req.DeckID, req.Name, req.CardIDs, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"deck": deck,
	}, nil
}

// CardUpgrade 处理卡牌升级协议。
func (h *BizHandler) CardUpgrade(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.CardUpgradeRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_upgrade payload"}
	}
	result, err := h.CardService.UpgradeCard(ctx, targetUID, req.CardID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.Online, *result.Player)
	}
	return result, nil
}
