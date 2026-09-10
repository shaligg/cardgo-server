package globalcore

import (
	"errors"
	"strconv"

	"github.com/bigfish/go_orm_1/internal/repo"
)

var (
	ErrInvalidReqID             = repo.ErrInvalidReqID
	ErrInvalidCursor            = errors.New("invalid cursor")
	ErrInvalidListLimit         = errors.New("invalid list limit")
	ErrPlayerNotFound           = repo.ErrSocialPlayerNotFound
	ErrCannotFriendSelf         = errors.New("cannot add self as friend")
	ErrFriendRequestExists      = errors.New("friend request already exists")
	ErrFriendRequestNotFound    = errors.New("friend request not found")
	ErrFriendRelationNotFound   = errors.New("friend relation not found")
	ErrAlreadyFriends           = errors.New("players are already friends")
	ErrInvalidGuildName         = errors.New("guild name must contain 2-20 characters")
	ErrGuildNameTaken           = repo.ErrGuildNameTaken
	ErrGuildNotFound            = repo.ErrGuildNotFound
	ErrAlreadyInGuild           = repo.ErrAlreadyInGuild
	ErrNotGuildMember           = repo.ErrNotGuildMember
	ErrGuildPermissionDenied    = repo.ErrGuildPermissionDenied
	ErrGuildApplicationExists   = repo.ErrGuildApplicationExists
	ErrGuildApplicationNotFound = repo.ErrGuildApplicationNotFound
	ErrInvalidChatChannel       = errors.New("invalid chat channel")
	ErrInvalidChatContent       = errors.New("chat content must contain 1-200 characters")
)

const (
	FriendStatusPending  = "pending"
	FriendStatusAccepted = "accepted"
)

// Core 聚合当前 GameServer 可使用的公共领域核心接口。
//
// globalcore 不是独立公共服进程，也不是单纯 remote client；它承载公共领域接口、
// DTO 和可复用规则。MVP 阶段可以同进程本地实现，未来可把部分实现替换为远程客户端。
type Core struct {
	Friend FriendService
	Guild  GuildService
	Chat   ChatService
	Rank   RankService
	Mail   MailService
	Notice NoticeService
}

func requireReqID(reqID string) error {
	if reqID == "" {
		return ErrInvalidReqID
	}
	return nil
}

func parsePage(cursor string, limit int) (uint64, int, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return 0, 0, ErrInvalidListLimit
	}
	if cursor == "" {
		return 0, limit, nil
	}
	value, err := strconv.ParseUint(cursor, 10, 64)
	if err != nil {
		return 0, 0, ErrInvalidCursor
	}
	return value, limit, nil
}

func formatCursor(cursor uint64) string {
	if cursor == 0 {
		return ""
	}
	return strconv.FormatUint(cursor, 10)
}
