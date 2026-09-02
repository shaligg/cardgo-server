package card

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func newTestCardService(t *testing.T) (Service, *repo.DBPlayerRepository) {
	t.Helper()
	db := testdb.OpenGame(t)
	dbRepo := repo.NewDBPlayerRepository(db)
	items, err := gamedata.NewCatalog([]gamedata.ItemConfig{
		{ItemID: gamedata.ItemIDGold, Key: "gold", StorageType: gamedata.StoragePlayerField, StorageKey: "gold", Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	data, err := gamedata.NewGameData(
		[]gamedata.CardConfig{
			testCard(10001),
			testCard(10002),
			testCard(10003),
			testCard(10004),
			testCard(10005),
			testCard(10006),
		},
		[]gamedata.OrderConfig{{
			OrderID:      1,
			Key:          "order",
			Name:         "订单",
			OrderType:    "normal",
			Requirements: []gamedata.ResourceAmount{{Resource: "bread", Count: 1}},
			Rewards:      []gamedata.RewardConfig{{ItemID: gamedata.ItemIDGold, Count: 1}},
		}},
		[]gamedata.LevelConfig{{
			LevelID:            1,
			Name:               "关卡",
			Chapter:            1,
			TurnLimit:          5,
			ActionPointPerTurn: 3,
			OrderSlots:         1,
			InitialOrders:      1,
			Goal:               gamedata.LevelGoal{GoalType: "complete_orders", Target: 1},
			FixedCards:         []int64{10001},
			OrderPool:          []gamedata.OrderPoolEntry{{OrderID: 1, Weight: 1}},
			FirstClearRewards:  []gamedata.RewardConfig{{ItemID: gamedata.ItemIDGold, Count: 1}},
			RepeatRewards:      []gamedata.RewardConfig{{ItemID: gamedata.ItemIDGold, Count: 1}},
		}},
		items,
	)
	if err != nil {
		t.Fatalf("NewGameData: %v", err)
	}
	assets := asset.Service{Items: items, Players: dbRepo, Inventory: dbRepo, TxPlayers: dbRepo, TxInventory: dbRepo}
	return Service{Repo: dbRepo, Assets: assets, Tx: idb.NewTxManager(db), Data: data}, dbRepo
}

func TestGetCardsCreatesDefaultCards(t *testing.T) {
	svc, _ := newTestCardService(t)
	result, err := svc.GetCards(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetCards returned error: %v", err)
	}
	if len(result.Cards) != DefaultDeckSize {
		t.Fatalf("cards = %d, want %d", len(result.Cards), DefaultDeckSize)
	}
	if result.Cards[0].CardID != 10001 || result.Cards[0].Level != 1 || result.Cards[0].Count != 1 {
		t.Fatalf("first card = %+v, want default card 10001 level 1 count 1", result.Cards[0])
	}
}

func TestSaveDeckRejectsDuplicateCard(t *testing.T) {
	svc, _ := newTestCardService(t)
	_, err := svc.SaveDeck(context.Background(), "u1", DefaultDeckID, "dup", []int64{10001, 10001}, "deck-r1")
	if !errors.Is(err, ErrInvalidDeck) {
		t.Fatalf("err = %v, want ErrInvalidDeck", err)
	}
}

func TestSaveDeckStoresLegalDeck(t *testing.T) {
	svc, _ := newTestCardService(t)
	deck, err := svc.SaveDeck(context.Background(), "u1", DefaultDeckID, "main", []int64{10001, 10002, 10003}, "deck-r1")
	if err != nil {
		t.Fatalf("SaveDeck returned error: %v", err)
	}
	if deck.DeckID != DefaultDeckID || len(deck.CardIDs) != 3 || deck.CardIDs[2] != 10003 {
		t.Fatalf("deck = %+v, want saved deck", deck)
	}
	retry, err := svc.SaveDeck(context.Background(), "u1", DefaultDeckID, "main", []int64{10001, 10002, 10003}, "deck-r1")
	if err != nil {
		t.Fatalf("retry SaveDeck returned error: %v", err)
	}
	if len(retry.CardIDs) != 3 {
		t.Fatalf("retry deck = %+v, want first result", retry)
	}
}

func TestUpgradeCardConsumesGoldAndLevelsUp(t *testing.T) {
	svc, dbRepo := newTestCardService(t)
	ctx := context.Background()
	if _, err := dbRepo.ChangeGold(ctx, "u1", 100, gamedata.ItemIDGold, "test.grant", "gold-r1"); err != nil {
		t.Fatalf("grant gold: %v", err)
	}
	result, err := svc.UpgradeCard(ctx, "u1", 10001, "card-r1")
	if err != nil {
		t.Fatalf("UpgradeCard returned error: %v", err)
	}
	if result.Card.Level != 2 || len(result.Costs) != 1 {
		t.Fatalf("upgrade result = %+v, want level 2 cost 50", result)
	}
	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 50 {
		t.Fatalf("gold = %d, want 50", player.Gold)
	}

}

func TestUpgradeCardRejectsMissingConfiguredCost(t *testing.T) {
	svc, dbRepo := newTestCardService(t)
	ctx := context.Background()
	if _, err := dbRepo.ChangeGold(ctx, "u1", 1000, gamedata.ItemIDGold, "test.grant", "gold-r1"); err != nil {
		t.Fatalf("grant gold: %v", err)
	}
	if _, err := svc.UpgradeCard(ctx, "u1", 10001, "card-r1"); err != nil {
		t.Fatalf("first UpgradeCard returned error: %v", err)
	}

	_, err := svc.UpgradeCard(ctx, "u1", 10001, "card-r2")
	if !errors.Is(err, repo.ErrInvalidAmount) {
		t.Fatalf("err = %v, want ErrInvalidAmount", err)
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 950 {
		t.Fatalf("gold = %d, want 950", player.Gold)
	}
	cards, err := dbRepo.GetCards(ctx, "u1")
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if cards[0].Level != 2 {
		t.Fatalf("card level = %d, want 2", cards[0].Level)
	}
}

func testCard(cardID int64) gamedata.CardConfig {
	return gamedata.CardConfig{
		CardID:   cardID,
		Key:      "card",
		Name:     "卡牌",
		Rarity:   "N",
		CardType: "material",
		Cost:     1,
		Effects:  []gamedata.EffectConfig{{EffectType: "gain_resource", Resource: "bread", Value: 1}},
		UpgradeCosts: []gamedata.CardUpgradeCostConfig{{
			TargetLevel: 2,
			Costs:       []gamedata.CostConfig{{ItemID: gamedata.ItemIDGold, Count: 50}},
		}},
	}
}
