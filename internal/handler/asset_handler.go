package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// AssetGrantItem 处理调试环境发放道具协议。
func (h *BizHandler) AssetGrantItem(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GrantItemRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid grant_item payload"}
	}
	res, err := h.AssetService.Grant(ctx, targetUID, []assetsvc.RewardItem{{ItemID: req.ItemID, Count: req.Count}}, "asset.grant_item", req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncAssetPlayerChanges(h.Online, res)
	return map[string]interface{}{
		"changes": res,
	}, nil
}

// AssetConsumeItem 处理调试环境消耗道具协议。
func (h *BizHandler) AssetConsumeItem(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ConsumeItemRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid consume_item payload"}
	}
	res, err := h.AssetService.Consume(ctx, targetUID, []assetsvc.CostItem{{ItemID: req.ItemID, Count: req.Count}}, "asset.consume_item", req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	syncAssetPlayerChanges(h.Online, res)
	return map[string]interface{}{
		"changes": res,
	}, nil
}

func syncAssetPlayerChanges(online *state.OnlineState, changes []assetsvc.ChangeResult) {
	for _, change := range changes {
		if change.Player != nil {
			syncOnlinePlayerState(online, *change.Player)
		}
	}
}

// AssetGetInventory 处理通用背包查询协议。
func (h *BizHandler) AssetGetInventory(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid get_inventory payload"}
	}
	items, err := h.InventoryService.GetInventory(ctx, targetUID)
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	return map[string]interface{}{
		"items": items,
	}, nil
}
