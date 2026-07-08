package app

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	inventorysvc "github.com/bigfish/go_orm_1/internal/game/inventory"
)

// assetHandler 处理资产与通用背包协议。
//
// 资产写入统一走 AssetService，背包查询走 InventoryService。
type assetHandler struct {
	assetService     assetsvc.Service
	inventoryService inventorysvc.Service
}

func newAssetHandler(assetService assetsvc.Service, inventoryService inventorysvc.Service) *assetHandler {
	return &assetHandler{assetService: assetService, inventoryService: inventoryService}
}

func (h *assetHandler) GrantItem(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.GrantItemRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid grant_item payload"}
	}
	res, err := h.assetService.Grant(ctx, targetUID, []assetsvc.RewardItem{{ItemID: req.ItemID, Count: req.Count}}, "asset.grant_item", req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"changes": res,
	}, nil
}

func (h *assetHandler) ConsumeItem(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.ConsumeItemRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid consume_item payload"}
	}
	res, err := h.assetService.Consume(ctx, targetUID, []assetsvc.CostItem{{ItemID: req.ItemID, Count: req.Count}}, "asset.consume_item", req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	return map[string]interface{}{
		"changes": res,
	}, nil
}

func (h *assetHandler) GetInventory(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid get_inventory payload"}
	}
	items, err := h.inventoryService.GetInventory(ctx, targetUID)
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	return map[string]interface{}{
		"items": items,
	}, nil
}
