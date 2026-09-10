package globalcore

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func TestLocalFriendServiceLifecycle(t *testing.T) {
	dbRepo := newSocialRepository(t, "friend_a", "friend_b")
	service := LocalFriendService{Repo: dbRepo}
	ctx := context.Background()

	if err := service.Apply(ctx, "friend_a", "friend_b", "friend-apply"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertFriendStatus(t, service, "friend_a", "friend_b", "outgoing")
	assertFriendStatus(t, service, "friend_b", "friend_a", "incoming")

	if err := service.Approve(ctx, "friend_b", "friend_a", "friend-approve"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	assertFriendStatus(t, service, "friend_a", "friend_b", repo.FriendStatusAccepted)
	assertFriendStatus(t, service, "friend_b", "friend_a", repo.FriendStatusAccepted)

	if err := service.Remove(ctx, "friend_a", "friend_b", "friend-remove"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	items, _, err := service.List(ctx, "friend_a", "", 20)
	if err != nil {
		t.Fatalf("List after remove: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("friend count after remove = %d, want 0", len(items))
	}
}

func TestLocalFriendServiceRejectsInvalidRelation(t *testing.T) {
	dbRepo := newSocialRepository(t, "friend_owner")
	service := LocalFriendService{Repo: dbRepo}
	ctx := context.Background()

	if err := service.Apply(ctx, "friend_owner", "friend_owner", "friend-self"); !errors.Is(err, ErrCannotFriendSelf) {
		t.Fatalf("self Apply error = %v, want %v", err, ErrCannotFriendSelf)
	}
	if err := service.Apply(ctx, "friend_owner", "missing_player", "friend-missing"); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("missing Apply error = %v, want %v", err, ErrPlayerNotFound)
	}
}

func TestLocalGuildAndChatLifecycle(t *testing.T) {
	dbRepo := newSocialRepository(t, "guild_owner", "guild_member", "guild_outsider")
	guilds := LocalGuildService{Repo: dbRepo}
	chat := LocalChatService{Messages: dbRepo, Membership: dbRepo}
	ctx := context.Background()

	guild, err := guilds.Create(ctx, "guild_owner", "猫咪工坊", "guild-create")
	if err != nil {
		t.Fatalf("Create guild: %v", err)
	}
	if guild.MemberCount != 1 || guild.MyRole != repo.GuildRoleLeader {
		t.Fatalf("created guild = %#v", guild)
	}

	if _, err := chat.SendChannelMsg(ctx, "guild", "guild_outsider", "越权消息", "chat-denied"); !errors.Is(err, ErrNotGuildMember) {
		t.Fatalf("outsider guild chat error = %v, want %v", err, ErrNotGuildMember)
	}
	if _, _, err := chat.PullHistory(ctx, "guild", "guild_outsider", "", 20); !errors.Is(err, ErrNotGuildMember) {
		t.Fatalf("outsider guild history error = %v, want %v", err, ErrNotGuildMember)
	}
	if err := guilds.ApplyJoin(ctx, "guild_member", guild.GuildID, "guild-apply"); err != nil {
		t.Fatalf("ApplyJoin: %v", err)
	}
	applications, _, err := guilds.ListApplications(ctx, "guild_owner", guild.GuildID, "", 20)
	if err != nil || len(applications) != 1 || applications[0].UID != "guild_member" {
		t.Fatalf("ListApplications = %#v err=%v", applications, err)
	}
	if _, _, err := guilds.ListApplications(ctx, "guild_outsider", guild.GuildID, "", 20); !errors.Is(err, ErrGuildPermissionDenied) {
		t.Fatalf("outsider ListApplications error = %v, want %v", err, ErrGuildPermissionDenied)
	}
	if err := guilds.ApproveJoin(ctx, "guild_outsider", guild.GuildID, "guild_member", "guild-denied"); !errors.Is(err, ErrGuildPermissionDenied) {
		t.Fatalf("outsider ApproveJoin error = %v, want %v", err, ErrGuildPermissionDenied)
	}
	if err := guilds.ApproveJoin(ctx, "guild_owner", guild.GuildID, "guild_member", "guild-approve"); err != nil {
		t.Fatalf("ApproveJoin: %v", err)
	}
	if _, err := guilds.Create(ctx, "guild_member", "第二公会", "second-guild"); !errors.Is(err, ErrAlreadyInGuild) {
		t.Fatalf("second Create error = %v, want %v", err, ErrAlreadyInGuild)
	}

	detail, err := guilds.Get(ctx, "guild_member", "")
	if err != nil {
		t.Fatalf("Get own guild: %v", err)
	}
	if detail.MemberCount != 2 || detail.MyRole != repo.GuildRoleMember || len(detail.Members) != 2 {
		t.Fatalf("guild detail = %#v", detail)
	}

	if _, err := chat.SendChannelMsg(ctx, "world", "guild_owner", "第一条世界消息", "world-1"); err != nil {
		t.Fatalf("Send world message 1: %v", err)
	}
	if _, err := chat.SendChannelMsg(ctx, "world", "guild_member", "第二条世界消息", "world-2"); err != nil {
		t.Fatalf("Send world message 2: %v", err)
	}
	if _, err := chat.SendChannelMsg(ctx, "world", "guild_owner", "第三条世界消息", "world-3"); err != nil {
		t.Fatalf("Send world message 3: %v", err)
	}
	latest, cursor, err := chat.PullHistory(ctx, "world", "guild_owner", "", 2)
	if err != nil {
		t.Fatalf("Pull latest world history: %v", err)
	}
	if len(latest) != 2 || latest[0].Content != "第二条世界消息" || latest[1].Content != "第三条世界消息" || cursor == "" {
		t.Fatalf("latest history = %#v cursor=%q", latest, cursor)
	}
	older, nextCursor, err := chat.PullHistory(ctx, "world", "guild_owner", cursor, 2)
	if err != nil {
		t.Fatalf("Pull older world history: %v", err)
	}
	if len(older) != 1 || older[0].Content != "第一条世界消息" || nextCursor != "" {
		t.Fatalf("older history = %#v cursor=%q", older, nextCursor)
	}

	guildMessage, err := chat.SendChannelMsg(ctx, "guild", "guild_member", "公会消息", "guild-chat")
	if err != nil {
		t.Fatalf("Send guild message: %v", err)
	}
	if guildMessage.ChannelID != "guild:"+guild.GuildID {
		t.Fatalf("guild channel = %q", guildMessage.ChannelID)
	}

	if err := guilds.Leave(ctx, "guild_owner", "guild-owner-leave"); err != nil {
		t.Fatalf("leader Leave: %v", err)
	}
	detail, err = guilds.Get(ctx, "guild_member", "")
	if err != nil {
		t.Fatalf("Get guild after leader leaves: %v", err)
	}
	if detail.OwnerUID != "guild_member" || detail.MyRole != repo.GuildRoleLeader || detail.MemberCount != 1 {
		t.Fatalf("transferred guild = %#v", detail)
	}
}

func assertFriendStatus(t *testing.T, service LocalFriendService, uid string, otherUID string, status string) {
	t.Helper()
	items, nextCursor, err := service.List(context.Background(), uid, "", 20)
	if err != nil {
		t.Fatalf("List %s: %v", uid, err)
	}
	if len(items) != 1 || items[0].UID != otherUID || items[0].Status != status || nextCursor != "" {
		t.Fatalf("List %s = %#v cursor=%q", uid, items, nextCursor)
	}
}

type socialTestRepository struct {
	*repo.DBPlayerRepository
	*repo.DBFriendRepository
	*repo.DBGuildRepository
	*repo.DBChatRepository
}

func newSocialRepository(t *testing.T, uids ...string) *socialTestRepository {
	t.Helper()
	db := testdb.OpenGame(t)
	dbRepo := &socialTestRepository{
		DBPlayerRepository: repo.NewDBPlayerRepository(db),
		DBFriendRepository: repo.NewDBFriendRepository(db),
		DBGuildRepository:  repo.NewDBGuildRepository(db),
		DBChatRepository:   repo.NewDBChatRepository(db),
	}
	for _, uid := range uids {
		if _, err := dbRepo.DBPlayerRepository.GetByUID(context.Background(), uid); err != nil {
			t.Fatalf("create player %s: %v", uid, err)
		}
	}
	return dbRepo
}
