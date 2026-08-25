package handler

import (
	"errors"
	"testing"

	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

func TestToBizErrorMapsExpectedClientCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "bad request", err: repo.ErrInvalidReqID, code: terrors.CodeBadRequest},
		{name: "not found", err: repo.ErrCardNotOwned, code: terrors.CodeNotFound},
		{name: "insufficient", err: repo.ErrInsufficientGold, code: terrors.CodeInsufficient},
		{name: "already max", err: repo.ErrCardMaxLevel, code: terrors.CodeAlreadyMax},
		{name: "precondition", err: battlesvc.ErrLevelNotComplete, code: terrors.CodePreconditionFailed},
		{name: "server config missing", err: cardsvc.ErrGameDataMissing, code: terrors.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toBizError(tt.err)
			if got.Code != tt.code {
				t.Fatalf("code = %s, want %s", got.Code, tt.code)
			}
		})
	}
}

func TestToBizErrorSupportsWrappedErrors(t *testing.T) {
	got := toBizError(errors.Join(errors.New("upgrade failed"), repo.ErrFacilityMaxLevel))
	if got.Code != terrors.CodeAlreadyMax {
		t.Fatalf("code = %s, want %s", got.Code, terrors.CodeAlreadyMax)
	}
}

func TestSyncAssetPlayerChangesUpdatesOnlineState(t *testing.T) {
	online := state.NewOnlineState()
	syncAssetPlayerChanges(online, []assetsvc.ChangeResult{
		{Item: &repo.InventoryItem{UID: "u1", ItemID: 2001, Count: 2}},
		{Player: &repo.Player{UID: "u1", Level: 2, Gold: 30}},
	})

	st, ok := online.Get("u1")
	if !ok || st.Data["gold"] != int64(30) || st.Data["level"] != 2 {
		t.Fatalf("online state = %+v, ok=%v", st, ok)
	}
}
