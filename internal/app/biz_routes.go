package app

import (
	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	assetsvc "github.com/bigfish/go_orm_1/internal/game/asset"
	battlesvc "github.com/bigfish/go_orm_1/internal/game/battle"
	cardsvc "github.com/bigfish/go_orm_1/internal/game/card"
	inventorysvc "github.com/bigfish/go_orm_1/internal/game/inventory"
	playersvc "github.com/bigfish/go_orm_1/internal/game/player"
	workshopsvc "github.com/bigfish/go_orm_1/internal/game/workshop"
	"github.com/bigfish/go_orm_1/internal/platform/state"
)

// newRegisteredBizRouter 创建并注册所有 WS 业务协议。
//
// 这里是统一协议绑定表；阅读本函数即可看到本期所有 op_code 对应的处理函数。
func newRegisteredBizRouter(playerService playersvc.Service, assetService assetsvc.Service, inventoryService inventorysvc.Service, cardService cardsvc.Service, battleService *battlesvc.Service, workshopService workshopsvc.Service, online *state.OnlineState, enableDebugOps bool) *bizRouter {
	router := newBizRouter()
	player := newPlayerHandler(playerService, online)
	asset := newAssetHandler(assetService, inventoryService)
	card := newCardHandler(cardService, online)
	level := newLevelHandler(battleService)
	workshop := newWorkshopHandler(workshopService, online)

	registerBizRoutes(router, player, asset, card, level, workshop, enableDebugOps)
	return router
}

func registerBizRoutes(router *bizRouter, player *playerHandler, asset *assetHandler, card *cardHandler, level *levelHandler, workshop *workshopHandler, enableDebugOps bool) {
	router.Register(protocol.OpPlayerGetProfile, player.GetProfile)

	router.Register(protocol.OpAssetGetInventory, asset.GetInventory)

	router.Register(protocol.OpCardGetCards, card.GetCards)
	router.Register(protocol.OpCardSaveDeck, card.SaveDeck)
	router.Register(protocol.OpCardUpgrade, card.Upgrade)

	router.Register(protocol.OpLevelStart, level.Start)
	router.Register(protocol.OpLevelPlayCard, level.PlayCard)
	router.Register(protocol.OpLevelSettle, level.Settle)

	router.Register(protocol.OpWorkshopGetOverview, workshop.GetOverview)
	router.Register(protocol.OpWorkshopUpgradeFacility, workshop.UpgradeFacility)
	router.Register(protocol.OpWorkshopClaimOffline, workshop.ClaimOfflineReward)

	if enableDebugOps {
		router.Register(protocol.OpPlayerAddGold, player.AddGold)
		router.Register(protocol.OpPlayerConsumeGold, player.ConsumeGold)
		router.Register(protocol.OpAssetGrantItem, asset.GrantItem)
		router.Register(protocol.OpAssetConsumeItem, asset.ConsumeItem)
	}
}
