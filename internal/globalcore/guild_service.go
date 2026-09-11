package globalcore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GuildMember 是公会成员列表 DTO。
type GuildMember struct {
	UID      string `json:"uid"`
	Role     string `json:"role"`
	JoinedAt int64  `json:"joined_at"`
}

// GuildInfo 是公会查询 DTO。
type GuildInfo struct {
	GuildID     string        `json:"guild_id"`
	Name        string        `json:"name"`
	OwnerUID    string        `json:"owner_uid"`
	MemberCount int           `json:"member_count"`
	MyRole      string        `json:"my_role,omitempty"`
	JoinStatus  string        `json:"join_status,omitempty"`
	Members     []GuildMember `json:"members,omitempty"`
}

// GuildApplication 是会长待审批列表 DTO。
type GuildApplication struct {
	UID       string `json:"uid"`
	Level     int    `json:"level"`
	Nickname  string `json:"nickname"`
	AvatarID  int64  `json:"avatar_id"`
	CreatedAt int64  `json:"created_at"`
}

// GuildService 定义公会公共领域能力。
//
// 公会数据以 DB/Redis 等共享存储为权威，不能依赖某个 GameServer 的连接对象或在线热状态。
type GuildService interface {
	Create(ctx context.Context, uid string, name string, reqID string) (GuildInfo, error)
	Search(ctx context.Context, uid string, keyword string, cursor string, limit int) ([]GuildInfo, string, error)
	Get(ctx context.Context, uid string, guildID string) (GuildInfo, error)
	ListApplications(ctx context.Context, operatorUID string, guildID string, cursor string, limit int) ([]GuildApplication, string, error)
	ApplyJoin(ctx context.Context, uid string, guildID string, reqID string) error
	ApproveJoin(ctx context.Context, operatorUID string, guildID string, targetUID string, reqID string) error
	Leave(ctx context.Context, uid string, reqID string) error
}

// LocalGuildService 是公会领域的同进程实现。
type LocalGuildService struct {
	Repo     *repo.DBGuildRepository
	Messages *repo.DBChatRepository
	Tx       idb.TxManager
}

// Create 创建公会并把当前玩家设为会长。
func (s LocalGuildService) Create(ctx context.Context, uid string, name string, reqID string) (GuildInfo, error) {
	name = strings.TrimSpace(name)
	if err := requireReqID(reqID); err != nil {
		return GuildInfo{}, err
	}
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 20 {
		return GuildInfo{}, ErrInvalidGuildName
	}
	guildID := uuid.NewString()
	err := s.withTransaction(ctx, func(store *repo.DBGuildRepository, _ *repo.DBChatRepository) error {
		if err := store.LockPlayer(ctx, uid); err != nil {
			return err
		}
		if _, err := store.GetMembership(ctx, uid); err == nil {
			return ErrAlreadyInGuild
		} else if !errors.Is(err, repo.ErrNotGuildMember) {
			return err
		}
		return store.CreateGuildData(ctx, repo.GuildRecord{
			GuildID: guildID, Name: name, OwnerUID: uid,
		}, repo.GuildMemberRecord{
			GuildID: guildID, UID: uid, Role: repo.GuildRoleLeader, JoinedAt: time.Now().UTC().Unix(),
		}, reqID)
	})
	if err != nil {
		return GuildInfo{}, err
	}
	return s.Get(ctx, uid, guildID)
}

// Search 按名称分页搜索公会。
func (s LocalGuildService) Search(ctx context.Context, uid string, keyword string, cursor string, limit int) ([]GuildInfo, string, error) {
	if s.Repo == nil {
		return nil, "", fmt.Errorf("guild repository is nil")
	}
	afterID, pageSize, err := parsePage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	rows, nextCursor, err := s.Repo.SearchGuilds(ctx, uid, strings.TrimSpace(keyword), afterID, pageSize)
	if err != nil {
		return nil, "", err
	}
	items := make([]GuildInfo, 0, len(rows))
	for _, row := range rows {
		items = append(items, toGuildInfo(row))
	}
	return items, formatCursor(nextCursor), nil
}

// Get 查询指定公会；guildID 为空时查询当前玩家所属公会。
func (s LocalGuildService) Get(ctx context.Context, uid string, guildID string) (GuildInfo, error) {
	if s.Repo == nil {
		return GuildInfo{}, fmt.Errorf("guild repository is nil")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		membership, err := s.Repo.GetMembership(ctx, uid)
		if err != nil {
			return GuildInfo{}, err
		}
		guildID = membership.GuildID
	}
	record, err := s.Repo.GetGuild(ctx, uid, guildID)
	if err != nil {
		return GuildInfo{}, err
	}
	return toGuildInfo(record), nil
}

// ListApplications 分页返回会长可审批的入会申请。
func (s LocalGuildService) ListApplications(ctx context.Context, operatorUID string, guildID string, cursor string, limit int) ([]GuildApplication, string, error) {
	afterID, pageSize, err := parsePage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, "", ErrGuildNotFound
	}
	if s.Repo == nil {
		return nil, "", fmt.Errorf("guild repository is nil")
	}
	operator, err := s.Repo.GetMembership(ctx, operatorUID)
	if err != nil || operator.GuildID != guildID || operator.Role != repo.GuildRoleLeader {
		if err != nil && !errors.Is(err, repo.ErrNotGuildMember) {
			return nil, "", err
		}
		return nil, "", ErrGuildPermissionDenied
	}
	rows, nextCursor, err := s.Repo.ListGuildApplications(ctx, guildID, afterID, pageSize)
	if err != nil {
		return nil, "", err
	}
	items := make([]GuildApplication, 0, len(rows))
	for _, row := range rows {
		items = append(items, GuildApplication{
			UID:       row.UID,
			Level:     row.Level,
			Nickname:  row.Nickname,
			AvatarID:  row.AvatarID,
			CreatedAt: row.CreatedAt,
		})
	}
	return items, formatCursor(nextCursor), nil
}

// ApplyJoin 提交入会申请。
func (s LocalGuildService) ApplyJoin(ctx context.Context, uid string, guildID string, reqID string) error {
	guildID = strings.TrimSpace(guildID)
	if err := requireReqID(reqID); err != nil {
		return err
	}
	if guildID == "" {
		return ErrGuildNotFound
	}
	return s.withTransaction(ctx, func(store *repo.DBGuildRepository, _ *repo.DBChatRepository) error {
		if err := store.LockPlayer(ctx, uid); err != nil {
			return err
		}
		if err := store.LockGuild(ctx, guildID); err != nil {
			return err
		}
		if _, err := store.GetMembership(ctx, uid); err == nil {
			return ErrAlreadyInGuild
		} else if !errors.Is(err, repo.ErrNotGuildMember) {
			return err
		}
		return store.CreateGuildApplicationData(ctx, guildID, uid, reqID)
	})
}

// ApproveJoin 由会长审批目标玩家的入会申请。
func (s LocalGuildService) ApproveJoin(ctx context.Context, operatorUID string, guildID string, targetUID string, reqID string) error {
	guildID = strings.TrimSpace(guildID)
	targetUID = strings.TrimSpace(targetUID)
	if err := requireReqID(reqID); err != nil {
		return err
	}
	if guildID == "" || targetUID == "" || operatorUID == targetUID {
		return ErrGuildApplicationNotFound
	}
	return s.withTransaction(ctx, func(store *repo.DBGuildRepository, _ *repo.DBChatRepository) error {
		if err := lockGuildPlayers(ctx, store, operatorUID, targetUID); err != nil {
			return err
		}
		if err := store.LockGuild(ctx, guildID); err != nil {
			return err
		}
		operator, err := store.GetMembership(ctx, operatorUID)
		if err != nil || operator.GuildID != guildID || operator.Role != repo.GuildRoleLeader {
			if err != nil && !errors.Is(err, repo.ErrNotGuildMember) {
				return err
			}
			return ErrGuildPermissionDenied
		}
		if err := store.GetGuildApplicationForUpdate(ctx, guildID, targetUID); err != nil {
			return err
		}
		if _, err := store.GetMembership(ctx, targetUID); err == nil {
			return ErrAlreadyInGuild
		} else if !errors.Is(err, repo.ErrNotGuildMember) {
			return err
		}
		return store.AddGuildMemberData(ctx, repo.GuildMemberRecord{
			GuildID: guildID, UID: targetUID, Role: repo.GuildRoleMember, JoinedAt: time.Now().UTC().Unix(),
		}, reqID)
	})
}

// Leave 退出当前公会。
func (s LocalGuildService) Leave(ctx context.Context, uid string, reqID string) error {
	if err := requireReqID(reqID); err != nil {
		return err
	}
	return s.withTransaction(ctx, func(store *repo.DBGuildRepository, messages *repo.DBChatRepository) error {
		if err := store.LockPlayer(ctx, uid); err != nil {
			return err
		}
		membership, err := store.GetMembership(ctx, uid)
		if err != nil {
			return err
		}
		if err := store.LockGuild(ctx, membership.GuildID); err != nil {
			return err
		}
		if membership.Role != repo.GuildRoleLeader {
			return store.RemoveGuildMemberData(ctx, uid)
		}

		successor, found, err := store.FindGuildSuccessorForUpdate(ctx, membership.GuildID, uid)
		if err != nil {
			return err
		}
		if !found {
			if messages == nil {
				return fmt.Errorf("chat repository is nil")
			}
			if err := messages.DeleteChannelMessages(ctx, guildChatChannelID(membership.GuildID)); err != nil {
				return err
			}
			return store.DeleteGuildData(ctx, membership.GuildID, uid)
		}
		return store.TransferGuildLeadershipData(ctx, membership.GuildID, uid, successor.UID)
	})
}

// withTransaction 为一次公会命令提供统一事务边界和事务内 Repository。
func (s LocalGuildService) withTransaction(ctx context.Context, fn func(*repo.DBGuildRepository, *repo.DBChatRepository) error) error {
	if s.Repo == nil {
		return fmt.Errorf("guild repository is nil")
	}
	return s.Tx.Do(ctx, func(tx *gorm.DB) error {
		var messages *repo.DBChatRepository
		if s.Messages != nil {
			messages = s.Messages.WithTx(tx)
		}
		return fn(s.Repo.WithTx(tx), messages)
	})
}

// lockGuildPlayers 按固定 UID 顺序加锁，避免多玩家命令出现相反锁序。
func lockGuildPlayers(ctx context.Context, store *repo.DBGuildRepository, uids ...string) error {
	sort.Strings(uids)
	for i, uid := range uids {
		if i > 0 && uid == uids[i-1] {
			continue
		}
		if err := store.LockPlayer(ctx, uid); err != nil {
			return err
		}
	}
	return nil
}

func toGuildInfo(record repo.GuildRecord) GuildInfo {
	joinStatus := "none"
	if record.MyRole != "" {
		joinStatus = "member"
	} else if record.HasApplication {
		joinStatus = "applied"
	}
	members := make([]GuildMember, 0, len(record.Members))
	for _, member := range record.Members {
		members = append(members, GuildMember{UID: member.UID, Role: member.Role, JoinedAt: member.JoinedAt})
	}
	return GuildInfo{
		GuildID:     record.GuildID,
		Name:        record.Name,
		OwnerUID:    record.OwnerUID,
		MemberCount: record.MemberCount,
		MyRole:      record.MyRole,
		JoinStatus:  joinStatus,
		Members:     members,
	}
}
