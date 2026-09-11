package handler

import (
	"errors"
	"testing"

	assetsvc "github.com/bigfish/go_orm_1/internal/domain/asset"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	battlesvc "github.com/bigfish/go_orm_1/internal/gameplay/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/gameplay/card"
	workshopsvc "github.com/bigfish/go_orm_1/internal/gameplay/workshop"
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
		{name: "insufficient", err: assetsvc.ErrInsufficientGold, code: terrors.CodeInsufficient},
		{name: "already max", err: cardsvc.ErrCardMaxLevel, code: terrors.CodeAlreadyMax},
		{name: "precondition", err: battlesvc.ErrLevelNotComplete, code: terrors.CodePreconditionFailed},
		{name: "battle in progress", err: battlesvc.ErrBattleInProgress, code: terrors.CodePreconditionFailed},
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
	got := toBizError(errors.Join(errors.New("upgrade failed"), workshopsvc.ErrFacilityMaxLevel))
	if got.Code != terrors.CodeAlreadyMax {
		t.Fatalf("code = %s, want %s", got.Code, terrors.CodeAlreadyMax)
	}
}
