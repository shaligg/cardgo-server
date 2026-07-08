package app

import (
	"errors"
	"testing"

	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
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
