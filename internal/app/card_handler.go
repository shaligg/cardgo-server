package app

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// cardHandler 处理卡牌库存、卡组和成长协议。
//
// Handler 只负责拆包取参数和调用 CardService，具体规则不写在协议层。
type cardHandler struct {
	cardService cardsvc.Service
	online      *state.OnlineState
}

func newCardHandler(cardService cardsvc.Service, online *state.OnlineState) *cardHandler {
	return &cardHandler{cardService: cardService, online: online}
}

func (h *cardHandler) GetCards(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_get_cards payload"}
	}
	result, err := h.cardService.GetCards(ctx, targetUID)
	if err != nil {
		return nil, toBizError(err)
	}
	return result, nil
}

func (h *cardHandler) SaveDeck(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.CardSaveDeckRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_save_deck payload"}
	}
	deck, err := h.cardService.SaveDeck(ctx, targetUID, req.DeckID, req.Name, req.CardIDs, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"deck": deck,
	}, nil
}

func (h *cardHandler) Upgrade(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.CardUpgradeRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid card_upgrade payload"}
	}
	result, err := h.cardService.UpgradeCard(ctx, targetUID, req.CardID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.online, *result.Player)
	}
	return result, nil
}
