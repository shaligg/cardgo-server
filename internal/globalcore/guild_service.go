package globalcore

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
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
	Repo repo.GuildRepository
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
	record, err := s.Repo.CreateGuild(ctx, uid, uuid.NewString(), name, reqID)
	if err != nil {
		return GuildInfo{}, err
	}
	return toGuildInfo(record), nil
}

// Search 按名称分页搜索公会。
func (s LocalGuildService) Search(ctx context.Context, uid string, keyword string, cursor string, limit int) ([]GuildInfo, string, error) {
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
	record, err := s.Repo.GetGuild(ctx, uid, strings.TrimSpace(guildID))
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
	rows, nextCursor, err := s.Repo.ListGuildApplications(ctx, operatorUID, guildID, afterID, pageSize)
	if err != nil {
		return nil, "", err
	}
	items := make([]GuildApplication, 0, len(rows))
	for _, row := range rows {
		items = append(items, GuildApplication{
			UID:       row.UID,
			Level:     row.Level,
			Nickname:  row.Nickname,
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
	return s.Repo.CreateGuildApplication(ctx, uid, guildID, reqID)
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
	return s.Repo.ApproveGuildApplication(ctx, operatorUID, guildID, targetUID, reqID)
}

// Leave 退出当前公会。
func (s LocalGuildService) Leave(ctx context.Context, uid string, reqID string) error {
	if err := requireReqID(reqID); err != nil {
		return err
	}
	return s.Repo.LeaveGuild(ctx, uid)
}

func toGuildInfo(record repo.GuildRecord) GuildInfo {
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
		JoinStatus:  record.JoinStatus,
		Members:     members,
	}
}
