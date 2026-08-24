package handler

import (
	"context"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
	inventorysvc "github.com/bigfish/go_orm_1/internal/game/inventory"
	playersvc "github.com/bigfish/go_orm_1/internal/game/player"
	workshopsvc "github.com/bigfish/go_orm_1/internal/game/workshop"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// WebSearcher 是网页搜索 Handler 依赖的外部查询能力。
type WebSearcher interface {
	Search(ctx context.Context, query string) (protocol.WebSearchResponse, error)
}

// BizHandler 是所有游戏业务协议共用的入口对象。
//
// 具体协议方法按业务模块拆在各个 *_handler.go 文件中，玩法规则仍由对应 Service 实现。
type BizHandler struct {
	PlayerService    playersvc.Service
	AssetService     assetsvc.Service
	InventoryService inventorysvc.Service
	CardService      cardsvc.Service
	BattleService    *battlesvc.Service
	WorkshopService  workshopsvc.Service
	Searcher         WebSearcher
	Online           *state.OnlineState
}

// NewRegisteredRouter 创建并注册所有游戏业务协议。
//
// 这里是统一协议绑定表；阅读本函数即可看到本期所有 op_code 对应的处理函数。
func NewRegisteredRouter(h *BizHandler, enableDebugOps bool) *Router {
	router := NewRouter()
	router.Register(protocol.OpPlayerGetProfile, h.PlayerGetProfile)

	router.Register(protocol.OpAssetGetInventory, h.AssetGetInventory)

	router.Register(protocol.OpCardGetCards, h.CardGetCards)
	router.Register(protocol.OpCardSaveDeck, h.CardSaveDeck)
	router.Register(protocol.OpCardUpgrade, h.CardUpgrade)

	router.Register(protocol.OpLevelStart, h.LevelStart)
	router.Register(protocol.OpLevelPlayCard, h.LevelPlayCard)
	router.Register(protocol.OpLevelSettle, h.LevelSettle)

	router.Register(protocol.OpWorkshopGetOverview, h.WorkshopGetOverview)
	router.Register(protocol.OpWorkshopUpgradeFacility, h.WorkshopUpgradeFacility)
	router.Register(protocol.OpWorkshopClaimOffline, h.WorkshopClaimOfflineReward)

	// 网页搜索等待外部 HTTP，不占用玩家分片锁。
	router.RegisterWithMode(protocol.OpWebSearch, h.WebSearch, ExecutionHandlerManaged)

	if enableDebugOps {
		router.Register(protocol.OpPlayerAddGold, h.PlayerAddGold)
		router.Register(protocol.OpPlayerConsumeGold, h.PlayerConsumeGold)
		router.Register(protocol.OpAssetGrantItem, h.AssetGrantItem)
		router.Register(protocol.OpAssetConsumeItem, h.AssetConsumeItem)
	}
	return router
}
