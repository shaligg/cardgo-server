package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// WorkshopGetOverview 处理工坊总览协议。
func (h *BizHandler) WorkshopGetOverview(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_get_overview payload"}
	}
	overview, err := h.WorkshopService.GetOverview(ctx, targetUID)
	if err != nil {
		return nil, toBizError(err)
	}
	return overview, nil
}

// WorkshopUpgradeFacility 处理工坊设施升级协议。
func (h *BizHandler) WorkshopUpgradeFacility(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.WorkshopUpgradeFacilityRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_upgrade_facility payload"}
	}
	result, err := h.WorkshopService.UpgradeFacility(ctx, targetUID, req.FacilityID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.Online, *result.Player)
	}
	return result, nil
}

// WorkshopClaimOfflineReward 处理工坊离线奖励领取协议。
func (h *BizHandler) WorkshopClaimOfflineReward(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.WorkshopClaimOfflineRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_claim_offline_reward payload"}
	}
	result, err := h.WorkshopService.ClaimOfflineReward(ctx, targetUID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.Online, *result.Player)
	}
	return result, nil
}
