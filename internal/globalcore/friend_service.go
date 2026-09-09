package globalcore

import (
	"context"
	"strings"

	"github.com/bigfish/go_orm_1/internal/repo"
)

// FriendItem 是好友列表返回的轻量 DTO。
type FriendItem struct {
	UID      string `json:"uid"`
	Level    int    `json:"level"`
	Nickname string `json:"nickname"`
	Status   string `json:"status"`
}

// LocalFriendService 是好友领域的同进程实现。
//
// 它只依赖持久化接口，未来 GameServer 可把 FriendService 替换为 RemoteClient。
type LocalFriendService struct {
	Repo repo.FriendRepository
}

// Apply 创建一条好友申请。
func (s LocalFriendService) Apply(ctx context.Context, uid string, targetUID string, reqID string) error {
	uid = strings.TrimSpace(uid)
	targetUID = strings.TrimSpace(targetUID)
	if err := requireReqID(reqID); err != nil {
		return err
	}
	if uid == "" || targetUID == "" {
		return ErrPlayerNotFound
	}
	if uid == targetUID {
		return ErrCannotFriendSelf
	}
	return s.Repo.CreateFriendRequest(ctx, uid, targetUID, reqID)
}

// Approve 同意 targetUID 发来的好友申请。
func (s LocalFriendService) Approve(ctx context.Context, uid string, targetUID string, reqID string) error {
	uid = strings.TrimSpace(uid)
	targetUID = strings.TrimSpace(targetUID)
	if err := requireReqID(reqID); err != nil {
		return err
	}
	if uid == "" || targetUID == "" || uid == targetUID {
		return ErrFriendRequestNotFound
	}
	return s.Repo.ApproveFriendRequest(ctx, uid, targetUID, reqID)
}

// Remove 删除好友关系或待处理申请。
func (s LocalFriendService) Remove(ctx context.Context, uid string, targetUID string, reqID string) error {
	uid = strings.TrimSpace(uid)
	targetUID = strings.TrimSpace(targetUID)
	if err := requireReqID(reqID); err != nil {
		return err
	}
	if uid == "" || targetUID == "" || uid == targetUID {
		return ErrFriendRelationNotFound
	}
	return s.Repo.DeleteFriendRelation(ctx, uid, targetUID)
}

// List 分页返回好友和申请状态。
func (s LocalFriendService) List(ctx context.Context, uid string, cursor string, limit int) ([]FriendItem, string, error) {
	afterID, pageSize, err := parsePage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	rows, nextCursor, err := s.Repo.ListFriendRelations(ctx, uid, afterID, pageSize)
	if err != nil {
		return nil, "", err
	}
	items := make([]FriendItem, 0, len(rows))
	for _, row := range rows {
		status := row.Status
		if row.Status == repo.FriendStatusPending {
			status = "incoming"
			if row.RequesterUID == uid {
				status = "outgoing"
			}
		}
		items = append(items, FriendItem{
			UID:      row.OtherUID,
			Level:    row.Level,
			Nickname: row.Nickname,
			Status:   status,
		})
	}
	return items, formatCursor(nextCursor), nil
}

// FriendService 定义好友公共领域能力。
//
// 好友关系属于跨玩家公共数据，接口必须保持 DTO 化，不能依赖 GameServer 连接、
// session 或在线热状态；未来可由 LocalService 替换为 RemoteClient。
type FriendService interface {
	Apply(ctx context.Context, uid string, targetUID string, reqID string) error
	Approve(ctx context.Context, uid string, targetUID string, reqID string) error
	Remove(ctx context.Context, uid string, targetUID string, reqID string) error
	List(ctx context.Context, uid string, cursor string, limit int) ([]FriendItem, string, error)
}
