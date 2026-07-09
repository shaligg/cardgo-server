package handler

import (
	"errors"

	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
	workshopsvc "github.com/bigfish/go_orm_1/internal/game/workshop"
	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo"
)

// toBizError 把内部错误转换成客户端可识别的业务错误码。
func toBizError(err error) *terrors.BizError {
	code := terrors.CodeInternal
	switch {
	case isInvalidRequestError(err):
		code = terrors.CodeBadRequest
	case isNotFoundError(err):
		code = terrors.CodeNotFound
	case isInsufficientResourceError(err):
		code = terrors.CodeInsufficient
	case isAlreadyMaxError(err):
		code = terrors.CodeAlreadyMax
	case isPreconditionFailedError(err):
		code = terrors.CodePreconditionFailed
	}
	return &terrors.BizError{Code: code, Msg: err.Error()}
}

func isInvalidRequestError(err error) bool {
	return errors.Is(err, repo.ErrInvalidReqID) ||
		errors.Is(err, repo.ErrInvalidAmount) ||
		errors.Is(err, assetsvc.ErrUnsupportedItemID) ||
		errors.Is(err, assetsvc.ErrBatchNotSupported) ||
		errors.Is(err, assetsvc.ErrUnsupportedStorage) ||
		errors.Is(err, battlesvc.ErrInvalidReqID) ||
		errors.Is(err, battlesvc.ErrCardNotInSession) ||
		errors.Is(err, cardsvc.ErrInvalidDeck)
}

func isNotFoundError(err error) bool {
	return errors.Is(err, repo.ErrCardNotOwned) ||
		errors.Is(err, repo.ErrDeckNotFound) ||
		errors.Is(err, battlesvc.ErrLevelNotFound) ||
		errors.Is(err, battlesvc.ErrSessionNotFound) ||
		errors.Is(err, battlesvc.ErrCardNotFound) ||
		errors.Is(err, cardsvc.ErrCardNotFound) ||
		errors.Is(err, workshopsvc.ErrFacilityNotFound)
}

func isInsufficientResourceError(err error) bool {
	return errors.Is(err, repo.ErrInsufficientGold) ||
		errors.Is(err, repo.ErrInsufficientItem) ||
		errors.Is(err, battlesvc.ErrInsufficientResource)
}

func isAlreadyMaxError(err error) bool {
	return errors.Is(err, repo.ErrCardMaxLevel) ||
		errors.Is(err, repo.ErrFacilityMaxLevel)
}

func isPreconditionFailedError(err error) bool {
	return errors.Is(err, battlesvc.ErrLevelNotComplete)
}

// syncOnlinePlayerState 把最新玩家快照写入在线热状态。
func syncOnlinePlayerState(online *state.OnlineState, p repo.Player) {
	if online == nil {
		return
	}
	prev, ok := online.Get(p.UID)
	version := int64(1)
	if ok {
		version = prev.Version + 1
	}
	online.Set(state.PlayerState{
		UID:     p.UID,
		Version: version,
		Data: map[string]interface{}{
			"uid":   p.UID,
			"level": p.Level,
			"gold":  p.Gold,
		},
	})
}
