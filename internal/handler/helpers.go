package handler

import (
	"errors"

	assetsvc "github.com/bigfish/go_orm_1/internal/domain/asset"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	battlesvc "github.com/bigfish/go_orm_1/internal/gameplay/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/gameplay/card"
	workshopsvc "github.com/bigfish/go_orm_1/internal/gameplay/workshop"
	"github.com/bigfish/go_orm_1/internal/globalcore"
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
		errors.Is(err, cardsvc.ErrInvalidDeck) ||
		errors.Is(err, globalcore.ErrInvalidCursor) ||
		errors.Is(err, globalcore.ErrInvalidListLimit) ||
		errors.Is(err, globalcore.ErrCannotFriendSelf) ||
		errors.Is(err, globalcore.ErrInvalidGuildName) ||
		errors.Is(err, globalcore.ErrInvalidChatChannel) ||
		errors.Is(err, globalcore.ErrInvalidChatContent)
}

func isNotFoundError(err error) bool {
	return errors.Is(err, repo.ErrCardNotOwned) ||
		errors.Is(err, repo.ErrDeckNotFound) ||
		errors.Is(err, battlesvc.ErrLevelNotFound) ||
		errors.Is(err, battlesvc.ErrSessionNotFound) ||
		errors.Is(err, battlesvc.ErrCardNotFound) ||
		errors.Is(err, cardsvc.ErrCardNotFound) ||
		errors.Is(err, workshopsvc.ErrFacilityNotFound) ||
		errors.Is(err, globalcore.ErrPlayerNotFound) ||
		errors.Is(err, globalcore.ErrFriendRequestNotFound) ||
		errors.Is(err, globalcore.ErrFriendRelationNotFound) ||
		errors.Is(err, globalcore.ErrGuildNotFound) ||
		errors.Is(err, globalcore.ErrGuildApplicationNotFound)
}

func isInsufficientResourceError(err error) bool {
	return errors.Is(err, assetsvc.ErrInsufficientGold) ||
		errors.Is(err, assetsvc.ErrInsufficientItem) ||
		errors.Is(err, battlesvc.ErrInsufficientResource)
}

func isAlreadyMaxError(err error) bool {
	return errors.Is(err, cardsvc.ErrCardMaxLevel) ||
		errors.Is(err, workshopsvc.ErrFacilityMaxLevel)
}

func isPreconditionFailedError(err error) bool {
	return errors.Is(err, battlesvc.ErrLevelNotComplete) ||
		errors.Is(err, battlesvc.ErrBattleInProgress) ||
		errors.Is(err, globalcore.ErrFriendRequestExists) ||
		errors.Is(err, globalcore.ErrAlreadyFriends) ||
		errors.Is(err, globalcore.ErrGuildNameTaken) ||
		errors.Is(err, globalcore.ErrAlreadyInGuild) ||
		errors.Is(err, globalcore.ErrNotGuildMember) ||
		errors.Is(err, globalcore.ErrGuildPermissionDenied) ||
		errors.Is(err, globalcore.ErrGuildApplicationExists)
}
