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

// CreateGuild 在一个事务中创建公会并把创建者设为会长。
func (r *DBPlayerRepository) CreateGuild(ctx context.Context, uid string, guildID string, name string, reqID string) (GuildRecord, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPlayer(tx, uid); err != nil {
			return err
		}
		var membershipCount int64
		if err := tx.Model(&model.GuildMember{}).Where("uid = ?", uid).Count(&membershipCount).Error; err != nil {
			return fmt.Errorf("check guild membership: %w", err)
		}
		if membershipCount > 0 {
			return ErrAlreadyInGuild
		}

		guild := model.Guild{GuildID: guildID, Name: name, OwnerUID: uid, ReqID: reqID}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&guild)
		if result.Error != nil {
			return fmt.Errorf("create guild: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGuildNameTaken
		}

		member := model.GuildMember{GuildID: guildID, UID: uid, Role: GuildRoleLeader, ReqID: reqID, JoinedAt: time.Now().UTC()}
		result = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&member)
		if result.Error != nil {
			return fmt.Errorf("create guild leader: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrAlreadyInGuild
		}
		if err := tx.Where("uid = ?", uid).Delete(&model.GuildApplication{}).Error; err != nil {
			return fmt.Errorf("delete old guild applications: %w", err)
		}
		return nil
	})
	if err != nil {
		return GuildRecord{}, err
	}
	return r.GetGuild(ctx, uid, guildID)
}

// SearchGuilds 按名称搜索公会，并返回当前玩家的成员或申请状态。
func (r *DBPlayerRepository) SearchGuilds(ctx context.Context, uid string, keyword string, afterID uint64, limit int) ([]GuildRecord, uint64, error) {
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

// GetGuild 查询指定公会详情；guildID 为空时查询玩家当前所属公会。
func (r *DBPlayerRepository) GetGuild(ctx context.Context, uid string, guildID string) (GuildRecord, error) {
	if guildID == "" {
		var membership model.GuildMember
		err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&membership).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return GuildRecord{}, ErrNotGuildMember
		}
		if err != nil {
			return GuildRecord{}, fmt.Errorf("query guild membership: %w", err)
		}
		guildID = membership.GuildID
	}

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
		records[0].Members = append(records[0].Members, GuildMemberRecord{
			UID:      member.UID,
			Role:     member.Role,
			JoinedAt: member.JoinedAt.Unix(),
		})
	}
	return records[0], nil
}

// ListGuildApplications 校验会长权限后分页查询待审批申请。
func (r *DBPlayerRepository) ListGuildApplications(ctx context.Context, operatorUID string, guildID string, afterID uint64, limit int) ([]GuildApplicationRecord, uint64, error) {
	var operator model.GuildMember
	err := r.db.WithContext(ctx).Where("guild_id = ? AND uid = ?", guildID, operatorUID).Take(&operator).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, 0, ErrGuildPermissionDenied
	}
	if err != nil {
		return nil, 0, fmt.Errorf("query guild operator: %w", err)
	}
	if operator.Role != GuildRoleLeader {
		return nil, 0, ErrGuildPermissionDenied
	}

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
	levels, err := r.playerLevels(ctx, uids)
	if err != nil {
		return nil, 0, err
	}
	result := make([]GuildApplicationRecord, 0, len(rows))
	for _, row := range rows {
		level := levels[row.UID]
		if level == 0 {
			level = 1
		}
		result = append(result, GuildApplicationRecord{
			UID:       row.UID,
			Level:     level,
			Nickname:  row.UID,
			CreatedAt: row.CreatedAt.Unix(),
		})
	}
	return result, nextCursor, nil
}

// CreateGuildApplication 创建一条待会长审批的入会申请。
func (r *DBPlayerRepository) CreateGuildApplication(ctx context.Context, uid string, guildID string, reqID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPlayer(tx, uid); err != nil {
			return err
		}
		var guildCount int64
		if err := tx.Model(&model.Guild{}).Where("guild_id = ?", guildID).Count(&guildCount).Error; err != nil {
			return fmt.Errorf("check guild exists: %w", err)
		}
		if guildCount == 0 {
			return ErrGuildNotFound
		}
		var memberCount int64
		if err := tx.Model(&model.GuildMember{}).Where("uid = ?", uid).Count(&memberCount).Error; err != nil {
			return fmt.Errorf("check guild membership: %w", err)
		}
		if memberCount > 0 {
			return ErrAlreadyInGuild
		}
		application := model.GuildApplication{GuildID: guildID, UID: uid, ReqID: reqID}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&application)
		if result.Error != nil {
			return fmt.Errorf("create guild application: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrGuildApplicationExists
		}
		return nil
	})
}

// ApproveGuildApplication 校验会长权限并把申请者加入公会。
func (r *DBPlayerRepository) ApproveGuildApplication(ctx context.Context, operatorUID string, guildID string, targetUID string, reqID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var operator model.GuildMember
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("guild_id = ? AND uid = ?", guildID, operatorUID).Take(&operator).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGuildPermissionDenied
		}
		if err != nil {
			return fmt.Errorf("query guild operator: %w", err)
		}
		if operator.Role != GuildRoleLeader {
			return ErrGuildPermissionDenied
		}
		if err := lockSocialPlayer(tx, targetUID); err != nil {
			return err
		}

		var application model.GuildApplication
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("guild_id = ? AND uid = ?", guildID, targetUID).Take(&application).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGuildApplicationNotFound
		}
		if err != nil {
			return fmt.Errorf("query guild application: %w", err)
		}

		member := model.GuildMember{GuildID: guildID, UID: targetUID, Role: GuildRoleMember, ReqID: reqID, JoinedAt: time.Now().UTC()}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&member)
		if result.Error != nil {
			return fmt.Errorf("create guild member: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrAlreadyInGuild
		}
		if err := tx.Where("uid = ?", targetUID).Delete(&model.GuildApplication{}).Error; err != nil {
			return fmt.Errorf("delete guild applications: %w", err)
		}
		return nil
	})
}

// LeaveGuild 退出公会；会长退出时转让给最早加入的成员，无其他成员则解散公会。
func (r *DBPlayerRepository) LeaveGuild(ctx context.Context, uid string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPlayer(tx, uid); err != nil {
			return err
		}
		var member model.GuildMember
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", uid).Take(&member).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotGuildMember
		}
		if err != nil {
			return fmt.Errorf("query guild member: %w", err)
		}

		if member.Role != GuildRoleLeader {
			return tx.Delete(&member).Error
		}

		var successor model.GuildMember
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("guild_id = ? AND uid <> ?", member.GuildID, uid).
			Order("joined_at ASC, id ASC").Take(&successor).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Where("guild_id = ?", member.GuildID).Delete(&model.GuildApplication{}).Error; err != nil {
				return fmt.Errorf("delete guild applications: %w", err)
			}
			if err := tx.Where("channel_id = ?", "guild:"+member.GuildID).Delete(&model.ChatMessage{}).Error; err != nil {
				return fmt.Errorf("delete guild chat: %w", err)
			}
			if err := tx.Delete(&member).Error; err != nil {
				return fmt.Errorf("delete guild leader: %w", err)
			}
			if err := tx.Where("guild_id = ?", member.GuildID).Delete(&model.Guild{}).Error; err != nil {
				return fmt.Errorf("delete guild: %w", err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("query guild successor: %w", err)
		}
		if err := tx.Model(&successor).Update("role", GuildRoleLeader).Error; err != nil {
			return fmt.Errorf("promote guild successor: %w", err)
		}
		if err := tx.Model(&model.Guild{}).Where("guild_id = ?", member.GuildID).Update("owner_uid", successor.UID).Error; err != nil {
			return fmt.Errorf("update guild owner: %w", err)
		}
		if err := tx.Delete(&member).Error; err != nil {
			return fmt.Errorf("delete old guild leader: %w", err)
		}
		return nil
	})
}

// GetGuildIDByUID 查询玩家当前所属公会，供公会聊天解析真实频道使用。
func (r *DBPlayerRepository) GetGuildIDByUID(ctx context.Context, uid string) (string, error) {
	var member model.GuildMember
	err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNotGuildMember
	}
	if err != nil {
		return "", fmt.Errorf("query guild membership: %w", err)
	}
	return member.GuildID, nil
}

func (r *DBPlayerRepository) buildGuildRecords(ctx context.Context, uid string, guilds []model.Guild) ([]GuildRecord, error) {
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
		status := "none"
		if roles[guild.GuildID] != "" {
			status = "member"
		} else if applied[guild.GuildID] {
			status = "applied"
		}
		result = append(result, GuildRecord{
			GuildID:     guild.GuildID,
			Name:        guild.Name,
			OwnerUID:    guild.OwnerUID,
			MemberCount: counts[guild.GuildID],
			MyRole:      roles[guild.GuildID],
			JoinStatus:  status,
		})
	}
	return result, nil
}

// lockSocialPlayer 串行化同一玩家跨公会的创建、申请、审批和退出操作。
func lockSocialPlayer(tx *gorm.DB, uid string) error {
	var player model.Player
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("uid").Where("uid = ?", uid).Take(&player).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrSocialPlayerNotFound
	}
	if err != nil {
		return fmt.Errorf("lock social player: %w", err)
	}
	return nil
}
