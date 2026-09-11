package protocol

// UIDRequest 是不需要额外参数的玩家请求。
//
// 普通 WS 业务目标玩家统一来自 auth ticket 绑定的 uid，payload 中的 uid 会被忽略。
type UIDRequest struct{}

type AddGoldRequest struct {
	Delta int64  `json:"delta,omitempty"`
	ReqID string `json:"req_id,omitempty"`
}

type ConsumeGoldRequest struct {
	Amount int64  `json:"amount,omitempty"`
	ReqID  string `json:"req_id,omitempty"`
}

type GrantItemRequest struct {
	ItemID int64  `json:"item_id,omitempty"`
	Count  int64  `json:"count,omitempty"`
	ReqID  string `json:"req_id,omitempty"`
}

type ConsumeItemRequest struct {
	ItemID int64  `json:"item_id,omitempty"`
	Count  int64  `json:"count,omitempty"`
	ReqID  string `json:"req_id,omitempty"`
}

type CardSaveDeckRequest struct {
	DeckID  int32   `json:"deck_id,omitempty"`
	Name    string  `json:"name,omitempty"`
	CardIDs []int64 `json:"card_ids,omitempty"`
	ReqID   string  `json:"req_id,omitempty"`
}

type CardUpgradeRequest struct {
	CardID int64  `json:"card_id,omitempty"`
	ReqID  string `json:"req_id,omitempty"`
}

type LevelStartRequest struct {
	LevelID int64  `json:"level_id,omitempty"`
	ReqID   string `json:"req_id,omitempty"`
}

type LevelPlayCardRequest struct {
	LevelSessionID string `json:"level_session_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	CardID         int64  `json:"card_id,omitempty"`
	ReqID          string `json:"req_id,omitempty"`
}

type LevelSettleRequest struct {
	LevelSessionID string `json:"level_session_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	ReqID          string `json:"req_id,omitempty"`
}

type LevelEndTurnRequest struct {
	LevelSessionID string `json:"level_session_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	ReqID          string `json:"req_id,omitempty"`
}

type WorkshopUpgradeFacilityRequest struct {
	FacilityID string `json:"facility_id,omitempty"`
	ReqID      string `json:"req_id,omitempty"`
}

type WorkshopClaimOfflineRequest struct {
	ReqID string `json:"req_id,omitempty"`
}

// WebSearchRequest 是网页搜索请求。
type WebSearchRequest struct {
	Query string `json:"query,omitempty"`
}

// WebSearchResult 是返回客户端的一条网页搜索结果。
type WebSearchResult struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// WebSearchResponse 是网页搜索响应；超时时丢弃外部结果并返回空列表。
type WebSearchResponse struct {
	Query    string            `json:"query"`
	Results  []WebSearchResult `json:"results"`
	TimedOut bool              `json:"timed_out"`
}

// FriendApplyRequest 是好友申请请求；操作玩家由鉴权 UID 决定，payload 只提交目标玩家。
type FriendApplyRequest struct {
	TargetUID string `json:"target_uid"`
	ReqID     string `json:"req_id"`
}

// FriendApproveRequest 是同意好友申请请求。
type FriendApproveRequest struct {
	TargetUID string `json:"target_uid"`
	ReqID     string `json:"req_id"`
}

// FriendRemoveRequest 是删除好友或撤销申请请求。
type FriendRemoveRequest struct {
	TargetUID string `json:"target_uid"`
	ReqID     string `json:"req_id"`
}

// FriendListRequest 是好友关系分页请求，cursor 由上次响应原样回传。
type FriendListRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// GuildCreateRequest 是创建公会请求。
type GuildCreateRequest struct {
	Name  string `json:"name"`
	ReqID string `json:"req_id"`
}

// GuildSearchRequest 是按名称搜索公会的分页请求。
type GuildSearchRequest struct {
	Keyword string `json:"keyword,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// GuildApplyJoinRequest 是申请加入公会请求。
type GuildApplyJoinRequest struct {
	GuildID string `json:"guild_id"`
	ReqID   string `json:"req_id"`
}

// GuildApproveJoinRequest 是会长审批入会申请请求。
type GuildApproveJoinRequest struct {
	GuildID   string `json:"guild_id"`
	TargetUID string `json:"target_uid"`
	ReqID     string `json:"req_id"`
}

// GuildLeaveRequest 是退出公会请求。
type GuildLeaveRequest struct {
	ReqID string `json:"req_id"`
}

// GuildGetRequest 查询指定公会；guild_id 为空时查询自己的公会。
type GuildGetRequest struct {
	GuildID string `json:"guild_id,omitempty"`
}

// GuildListApplicationsRequest 是会长查询待审批申请的分页请求。
type GuildListApplicationsRequest struct {
	GuildID string `json:"guild_id"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// ChatSendRequest 向世界或当前公会频道发送消息。
type ChatSendRequest struct {
	Channel string `json:"channel"`
	Content string `json:"content"`
	ReqID   string `json:"req_id"`
}

// ChatHistoryRequest 拉取世界或当前公会频道历史消息。
type ChatHistoryRequest struct {
	Channel string `json:"channel"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}
