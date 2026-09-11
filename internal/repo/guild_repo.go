package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBGuildRepository 是基于 GORM 的公会、成员和申请仓储。
//
// 本类型只负责查询、事务内 CRUD 和唯一键冲突转换；公会权限与成员规则由 globalcore 处理。
type DBGuildRepository struct {
	db *gorm.DB
}

// NewDBGuildRepository 创建公会仓储。
func NewDBGuildRepository(db *gorm.DB) *DBGuildRepository {
	return &DBGuildRepository{db: db}
}

// WithTx 返回绑定到指定事务的公会仓储。
func (r *DBGuildRepository) WithTx(tx *gorm.DB) *DBGuildRepository {
	return &DBGuildRepository{db: tx}
}

// LockPlayer 锁定玩家基础行，串行化同一玩家跨节点的公会成员变更。
func (r *DBGuildRepository) LockPlayer(ctx context.Context, uid string) error {
	var player model.Player
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("uid").Where("uid = ?", uid).Take(&player).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSocialPlayerNotFound
	}
	if err != nil {
		return fmt.Errorf("lock guild player: %w", err)
	}
	return nil
}

// LockGuild 锁定公会主体行，避免审批、退出与解散交叉提交。
func (r *DBGuildRepository) LockGuild(ctx context.Context, guildID string) error {
	var guild model.Guild
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Select("guild_id").Where("guild_id = ?", guildID).Take(&guild).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrGuildNotFound
	}
	if err != nil {
		return fmt.Errorf("lock guild: %w", err)
	}
	return nil
}

// GetMembership 查询玩家当前唯一的公会成员记录。
func (r *DBGuildRepository) GetMembership(ctx context.Context, uid string) (GuildMemberRecord, error) {
	var member model.GuildMember
	err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GuildMemberRecord{}, ErrNotGuildMember
	}
	if err != nil {
		return GuildMemberRecord{}, fmt.Errorf("query guild membership: %w", err)
	}
	return toGuildMemberRecord(member), nil
}

// GetGuildApplicationForUpdate 锁定并确认指定入会申请存在。
func (r *DBGuildRepository) GetGuildApplicationForUpdate(ctx context.Context, guildID string, uid string) error {
	var application model.GuildApplication
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ? AND uid = ?", guildID, uid).Take(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrGuildApplicationNotFound
	}
	if err != nil {
		return fmt.Errorf("query guild application: %w", err)
	}
	return nil
}

// FindGuildSuccessorForUpdate 锁定并返回最早加入的其他成员。
func (r *DBGuildRepository) FindGuildSuccessorForUpdate(ctx context.Context, guildID string, excludeUID string) (GuildMemberRecord, bool, error) {
	var successor model.GuildMember
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ? AND uid <> ?", guildID, excludeUID).
		Order("joined_at ASC, id ASC").Take(&successor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GuildMemberRecord{}, false, nil
	}
	if err != nil {
		return GuildMemberRecord{}, false, fmt.Errorf("query guild successor: %w", err)
	}
	return toGuildMemberRecord(successor), true, nil
}

// CreateGuildData 写入公会主体和首位成员；调用方负责业务校验和事务边界。
func (r *DBGuildRepository) CreateGuildData(ctx context.Context, guild GuildRecord, leader GuildMemberRecord, reqID string) error {
	guildRow := model.Guild{GuildID: guild.GuildID, Name: guild.Name, OwnerUID: guild.OwnerUID, ReqID: reqID}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&guildRow)
	if result.Error != nil {
		return fmt.Errorf("create guild: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGuildNameTaken
	}

	memberRow := guildMemberModel(leader, reqID)
	result = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&memberRow)
	if result.Error != nil {
		return fmt.Errorf("create guild leader: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAlreadyInGuild
	}
	return r.DeleteGuildApplicationsByUID(ctx, leader.UID)
}

// CreateGuildApplicationData 写入一条入会申请；调用方负责存在性和成员状态校验。
func (r *DBGuildRepository) CreateGuildApplicationData(ctx context.Context, guildID string, uid string, reqID string) error {
	application := model.GuildApplication{GuildID: guildID, UID: uid, ReqID: reqID}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&application)
	if result.Error != nil {
		return fmt.Errorf("create guild application: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrGuildApplicationExists
	}
	return nil
}

// AddGuildMemberData 写入成员并删除该玩家的所有待处理申请。
func (r *DBGuildRepository) AddGuildMemberData(ctx context.Context, member GuildMemberRecord, reqID string) error {
	row := guildMemberModel(member, reqID)
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return fmt.Errorf("create guild member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAlreadyInGuild
	}
	return r.DeleteGuildApplicationsByUID(ctx, member.UID)
}

// DeleteGuildApplicationsByUID 删除玩家的全部待处理入会申请。
func (r *DBGuildRepository) DeleteGuildApplicationsByUID(ctx context.Context, uid string) error {
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).Delete(&model.GuildApplication{}).Error; err != nil {
		return fmt.Errorf("delete guild applications: %w", err)
	}
	return nil
}

// RemoveGuildMemberData 删除一名普通成员。
func (r *DBGuildRepository) RemoveGuildMemberData(ctx context.Context, uid string) error {
	if err := r.db.WithContext(ctx).Where("uid = ?", uid).Delete(&model.GuildMember{}).Error; err != nil {
		return fmt.Errorf("delete guild member: %w", err)
	}
	return nil
}

// TransferGuildLeadershipData 更新新会长并删除原会长成员记录。
func (r *DBGuildRepository) TransferGuildLeadershipData(ctx context.Context, guildID string, oldUID string, newUID string) error {
	if err := r.db.WithContext(ctx).Model(&model.GuildMember{}).
		Where("guild_id = ? AND uid = ?", guildID, newUID).Update("role", GuildRoleLeader).Error; err != nil {
		return fmt.Errorf("promote guild successor: %w", err)
	}
	if err := r.db.WithContext(ctx).Model(&model.Guild{}).
		Where("guild_id = ?", guildID).Update("owner_uid", newUID).Error; err != nil {
		return fmt.Errorf("update guild owner: %w", err)
	}
	return r.RemoveGuildMemberData(ctx, oldUID)
}

// DeleteGuildData 删除空公会、待处理申请和最后一名成员。
func (r *DBGuildRepository) DeleteGuildData(ctx context.Context, guildID string, leaderUID string) error {
	if err := r.db.WithContext(ctx).Where("guild_id = ?", guildID).Delete(&model.GuildApplication{}).Error; err != nil {
		return fmt.Errorf("delete guild applications: %w", err)
	}
	if err := r.RemoveGuildMemberData(ctx, leaderUID); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("guild_id = ?", guildID).Delete(&model.Guild{}).Error; err != nil {
		return fmt.Errorf("delete guild: %w", err)
	}
	return nil
}

// SearchGuilds 按名称搜索公会，并返回当前玩家的成员或申请状态。
func (r *DBGuildRepository) SearchGuilds(ctx context.Context, uid string, keyword string, afterID uint64, limit int) ([]GuildRecord, uint64, error) {
	var rows []model.Guild
	query := r.db.WithContext(ctx).Where("id > ?", afterID)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("search guilds: %w", err)
	}
	nextCursor := uint64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		nextCursor = rows[len(rows)-1].ID
	}
	records, err := r.buildGuildRecords(ctx, uid, rows)
	return records, nextCursor, err
}

// GetGuild 查询指定公会详情。
func (r *DBGuildRepository) GetGuild(ctx context.Context, uid string, guildID string) (GuildRecord, error) {
	var guild model.Guild
	err := r.db.WithContext(ctx).Where("guild_id = ?", guildID).Take(&guild).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return GuildRecord{}, ErrGuildNotFound
	}
	if err != nil {
		return GuildRecord{}, fmt.Errorf("query guild: %w", err)
	}
	records, err := r.buildGuildRecords(ctx, uid, []model.Guild{guild})
	if err != nil {
		return GuildRecord{}, err
	}

	var members []model.GuildMember
	if err := r.db.WithContext(ctx).
		Where("guild_id = ?", guildID).
		Order("CASE WHEN role = 'leader' THEN 0 ELSE 1 END, joined_at ASC, id ASC").
		Find(&members).Error; err != nil {
		return GuildRecord{}, fmt.Errorf("list guild members: %w", err)
	}
	records[0].Members = make([]GuildMemberRecord, 0, len(members))
	for _, member := range members {
		records[0].Members = append(records[0].Members, toGuildMemberRecord(member))
	}
	return records[0], nil
}

// ListGuildApplications 分页查询指定公会的待审批申请，不包含权限判断。
func (r *DBGuildRepository) ListGuildApplications(ctx context.Context, guildID string, afterID uint64, limit int) ([]GuildApplicationRecord, uint64, error) {
	var rows []model.GuildApplication
	if err := r.db.WithContext(ctx).Where("guild_id = ? AND id > ?", guildID, afterID).
		Order("id ASC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list guild applications: %w", err)
	}
	nextCursor := uint64(0)
	if len(rows) > limit {
		rows = rows[:limit]
		nextCursor = rows[len(rows)-1].ID
	}
	uids := make([]string, 0, len(rows))
	for _, row := range rows {
		uids = append(uids, row.UID)
	}
	profiles, err := playerProfiles(ctx, r.db, uids)
	if err != nil {
		return nil, 0, err
	}
	result := make([]GuildApplicationRecord, 0, len(rows))
	for _, row := range rows {
		profile, ok := profiles[row.UID]
		if !ok {
			return nil, 0, fmt.Errorf("%w: %s", ErrSocialPlayerNotFound, row.UID)
		}
		result = append(result, GuildApplicationRecord{
			UID:       row.UID,
			Level:     profile.Level,
			Nickname:  profile.Nickname,
			AvatarID:  profile.AvatarID,
			CreatedAt: row.CreatedAt.Unix(),
		})
	}
	return result, nextCursor, nil
}

// GetGuildIDByUID 查询玩家当前所属公会，供公会聊天解析真实频道使用。
func (r *DBGuildRepository) GetGuildIDByUID(ctx context.Context, uid string) (string, error) {
	membership, err := r.GetMembership(ctx, uid)
	if err != nil {
		return "", err
	}
	return membership.GuildID, nil
}

// buildGuildRecords 批量补齐公会人数以及查询者的成员、申请状态。
func (r *DBGuildRepository) buildGuildRecords(ctx context.Context, uid string, guilds []model.Guild) ([]GuildRecord, error) {
	if len(guilds) == 0 {
		return []GuildRecord{}, nil
	}
	guildIDs := make([]string, 0, len(guilds))
	for _, guild := range guilds {
		guildIDs = append(guildIDs, guild.GuildID)
	}

	type guildCount struct {
		GuildID string
		Count   int
	}
	var countRows []guildCount
	if err := r.db.WithContext(ctx).Model(&model.GuildMember{}).
		Select("guild_id, COUNT(*) AS count").Where("guild_id IN ?", guildIDs).
		Group("guild_id").Scan(&countRows).Error; err != nil {
		return nil, fmt.Errorf("count guild members: %w", err)
	}
	counts := make(map[string]int, len(countRows))
	for _, row := range countRows {
		counts[row.GuildID] = row.Count
	}

	var memberships []model.GuildMember
	if err := r.db.WithContext(ctx).Where("uid = ? AND guild_id IN ?", uid, guildIDs).Find(&memberships).Error; err != nil {
		return nil, fmt.Errorf("query viewer guild roles: %w", err)
	}
	roles := make(map[string]string, len(memberships))
	for _, membership := range memberships {
		roles[membership.GuildID] = membership.Role
	}

	var applications []model.GuildApplication
	if err := r.db.WithContext(ctx).Where("uid = ? AND guild_id IN ?", uid, guildIDs).Find(&applications).Error; err != nil {
		return nil, fmt.Errorf("query viewer guild applications: %w", err)
	}
	applied := make(map[string]bool, len(applications))
	for _, application := range applications {
		applied[application.GuildID] = true
	}

	result := make([]GuildRecord, 0, len(guilds))
	for _, guild := range guilds {
		result = append(result, GuildRecord{
			GuildID:        guild.GuildID,
			Name:           guild.Name,
			OwnerUID:       guild.OwnerUID,
			MemberCount:    counts[guild.GuildID],
			MyRole:         roles[guild.GuildID],
			HasApplication: applied[guild.GuildID],
		})
	}
	return result, nil
}

func guildMemberModel(member GuildMemberRecord, reqID string) model.GuildMember {
	return model.GuildMember{
		GuildID:  member.GuildID,
		UID:      member.UID,
		Role:     member.Role,
		ReqID:    reqID,
		JoinedAt: time.Unix(member.JoinedAt, 0).UTC(),
	}
}

func toGuildMemberRecord(member model.GuildMember) GuildMemberRecord {
	return GuildMemberRecord{
		GuildID:  member.GuildID,
		UID:      member.UID,
		Role:     member.Role,
		JoinedAt: member.JoinedAt.Unix(),
	}
}
