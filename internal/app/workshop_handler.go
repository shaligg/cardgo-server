package app

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	workshopsvc "github.com/bigfish/go_orm_1/internal/game/workshop"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// workshopHandler 处理工坊协议。
//
// Handler 只负责拆包取参数和调用 WorkshopService，具体工坊规则不写在协议层。
type workshopHandler struct {
	workshopService workshopsvc.Service
	online          *state.OnlineState
}

func newWorkshopHandler(workshopService workshopsvc.Service, online *state.OnlineState) *workshopHandler {
	return &workshopHandler{workshopService: workshopService, online: online}
}

func (h *workshopHandler) GetOverview(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.UIDRequest
	if len(payload) > 0 && json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_get_overview payload"}
	}
	overview, err := h.workshopService.GetOverview(ctx, targetUID)
	if err != nil {
		return nil, toBizError(err)
	}
	return overview, nil
}

func (h *workshopHandler) UpgradeFacility(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.WorkshopUpgradeFacilityRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_upgrade_facility payload"}
	}
	result, err := h.workshopService.UpgradeFacility(ctx, targetUID, req.FacilityID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.online, *result.Player)
	}
	return result, nil
}

func (h *workshopHandler) ClaimOfflineReward(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.WorkshopClaimOfflineRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid workshop_claim_offline_reward payload"}
	}
	result, err := h.workshopService.ClaimOfflineReward(ctx, targetUID, req.ReqID)
	if err != nil {
		return nil, toBizError(err)
	}
	if result.Player != nil {
		syncOnlinePlayerState(h.online, *result.Player)
	}
	return result, nil
}
