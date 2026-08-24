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
