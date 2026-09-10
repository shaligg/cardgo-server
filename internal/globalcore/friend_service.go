package globalcore

import (
	"context"
	"fmt"
	"strings"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"gorm.io/gorm"
)

// FriendItem 是好友列表返回的轻量 DTO。
type FriendItem struct {
	UID      string `json:"uid"`
	Level    int    `json:"level"`
	Nickname string `json:"nickname"`
	AvatarID int64  `json:"avatar_id"`
	Status   string `json:"status"`
}

// LocalFriendService 是好友领域的同进程实现。
//
// 它负责好友规则与事务编排，未来 GameServer 可把 FriendService 替换为 RemoteClient。
type LocalFriendService struct {
	Repo *repo.DBFriendRepository
	Tx   idb.TxManager
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
	if s.Repo == nil {
		return fmt.Errorf("friend repository is nil")
	}
	exists, err := s.Repo.PlayerExists(ctx, targetUID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrPlayerNotFound
	}
	created, err := s.Repo.CreateFriendRelationData(ctx, uid, targetUID, uid, FriendStatusPending, reqID)
	if err != nil {
		return err
	}
	if created {
		return nil
	}
	relation, found, err := s.Repo.FindFriendRelation(ctx, uid, targetUID)
	if err != nil {
		return err
	}
	if found && relation.Status == FriendStatusAccepted {
		return ErrAlreadyFriends
	}
	return ErrFriendRequestExists
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
	return s.withTransaction(ctx, func(store *repo.DBFriendRepository) error {
		relation, found, err := store.FindFriendRelationForUpdate(ctx, uid, targetUID)
		if err != nil {
			return err
		}
		if !found {
			return ErrFriendRequestNotFound
		}
		if relation.Status == FriendStatusAccepted {
			return ErrAlreadyFriends
		}
		if relation.RequesterUID != targetUID {
			return ErrFriendRequestNotFound
		}
		return store.UpdateFriendRelationData(ctx, uid, targetUID, FriendStatusAccepted, reqID)
	})
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
	if s.Repo == nil {
		return fmt.Errorf("friend repository is nil")
	}
	deleted, err := s.Repo.DeleteFriendRelationData(ctx, uid, targetUID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrFriendRelationNotFound
	}
	return nil
}

// List 分页返回好友和申请状态。
func (s LocalFriendService) List(ctx context.Context, uid string, cursor string, limit int) ([]FriendItem, string, error) {
	afterID, pageSize, err := parsePage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	if s.Repo == nil {
		return nil, "", fmt.Errorf("friend repository is nil")
	}
	rows, nextCursor, err := s.Repo.ListFriendRelations(ctx, uid, afterID, pageSize)
	if err != nil {
		return nil, "", err
	}
	items := make([]FriendItem, 0, len(rows))
	for _, row := range rows {
		status := row.Status
		if row.Status == FriendStatusPending {
			status = "incoming"
			if row.RequesterUID == uid {
				status = "outgoing"
			}
		}
		items = append(items, FriendItem{
			UID:      row.OtherUID,
			Level:    row.Level,
			Nickname: row.Nickname,
			AvatarID: row.AvatarID,
			Status:   status,
		})
	}
	return items, formatCursor(nextCursor), nil
}

// withTransaction 为一次好友状态流转提供统一事务边界。
func (s LocalFriendService) withTransaction(ctx context.Context, fn func(*repo.DBFriendRepository) error) error {
	if s.Repo == nil {
		return fmt.Errorf("friend repository is nil")
	}
	return s.Tx.Do(ctx, func(tx *gorm.DB) error {
		return fn(s.Repo.WithTx(tx))
	})
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
