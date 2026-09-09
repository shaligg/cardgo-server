package model

import "time"

// FriendRelation 用一行保存两个玩家之间的申请或好友关系。
//
// UIDLow 和 UIDHigh 始终按字符串升序写入，保证同一对玩家只有一条关系记录。
type FriendRelation struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	UIDLow       string `gorm:"size:64;not null;uniqueIndex:uk_friend_pair"`
	UIDHigh      string `gorm:"size:64;not null;uniqueIndex:uk_friend_pair"`
	RequesterUID string `gorm:"size:64;not null;index"`
	Status       string `gorm:"size:16;not null;index"`
	ReqID        string `gorm:"size:128;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Guild 保存公会主体信息，GuildID 是对外稳定 ID，ID 只用于数据库分页。
type Guild struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	GuildID   string `gorm:"size:36;not null;uniqueIndex"`
	Name      string `gorm:"size:64;not null;uniqueIndex"`
	OwnerUID  string `gorm:"size:64;not null;index"`
	ReqID     string `gorm:"size:128;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GuildMember 保存玩家所属公会和职位；UID 唯一约束保证一名玩家只能加入一个公会。
type GuildMember struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	GuildID   string    `gorm:"size:36;not null;index:idx_guild_joined,priority:1"`
	UID       string    `gorm:"size:64;not null;uniqueIndex:uk_guild_member_uid"`
	Role      string    `gorm:"size:16;not null"`
	ReqID     string    `gorm:"size:128;not null"`
	JoinedAt  time.Time `gorm:"not null;index:idx_guild_joined,priority:2"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GuildApplication 保存待审批的入会申请。
type GuildApplication struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	GuildID   string `gorm:"size:36;not null;uniqueIndex:uk_guild_application"`
	UID       string `gorm:"size:64;not null;uniqueIndex:uk_guild_application;index"`
	ReqID     string `gorm:"size:128;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ChatMessage 保存世界和公会频道的短文本消息。
type ChatMessage struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	MsgID     string    `gorm:"size:36;not null;uniqueIndex"`
	ChannelID string    `gorm:"size:80;not null;index:idx_chat_channel_id,priority:1"`
	UID       string    `gorm:"size:64;not null;index"`
	Content   string    `gorm:"size:800;not null"`
	ReqID     string    `gorm:"size:128;not null;index"`
	CreatedAt time.Time `gorm:"index:idx_chat_channel_id,priority:2"`
}
