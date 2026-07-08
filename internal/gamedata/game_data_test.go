package gamedata

import "testing"

func TestLoadGameData(t *testing.T) {
	items, err := LoadItemCatalog("../../configs/gamedata/items.json")
	if err != nil {
		t.Fatalf("LoadItemCatalog returned error: %v", err)
	}

	data, err := LoadGameData(ConfigPaths{
		CardConfigPath:  "../../configs/gamedata/cards.json",
		OrderConfigPath: "../../configs/gamedata/orders.json",
		LevelConfigPath: "../../configs/gamedata/levels.json",
	}, items)
	if err != nil {
		t.Fatalf("LoadGameData returned error: %v", err)
	}
	if len(data.Cards) != 20 {
		t.Fatalf("cards = %d, want 20", len(data.Cards))
	}
	if len(data.Orders) != 10 {
		t.Fatalf("orders = %d, want 10", len(data.Orders))
	}
	if len(data.Levels) != 10 {
		t.Fatalf("levels = %d, want 10", len(data.Levels))
	}
}

func TestNewGameDataRejectsMissingOrderReference(t *testing.T) {
	items, err := NewCatalog([]ItemConfig{
		{ItemID: ItemIDGold, Key: "gold", StorageType: StoragePlayerField, StorageKey: "gold", Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog returned error: %v", err)
	}

	_, err = NewGameData(
		[]CardConfig{{CardID: 1, Key: "card", Name: "card", Rarity: "N", CardType: "material", Effects: []EffectConfig{{EffectType: "gain_resource"}}}},
		[]OrderConfig{{OrderID: 1, Key: "order", Name: "order", OrderType: "normal", Requirements: []ResourceAmount{{Resource: "bread", Count: 1}}, Rewards: []RewardConfig{{ItemID: ItemIDGold, Count: 1}}}},
		[]LevelConfig{{LevelID: 1, Name: "level", Chapter: 1, TurnLimit: 1, ActionPointPerTurn: 1, OrderSlots: 1, InitialOrders: 1, Goal: LevelGoal{GoalType: "complete_orders", Target: 1}, OrderPool: []OrderPoolEntry{{OrderID: 999, Weight: 1}}}},
		items,
	)
	if err == nil {
		t.Fatalf("expected missing order reference error")
	}
}

func TestNewGameDataRejectsDuplicateCardID(t *testing.T) {
	items := mustTestItemCatalog(t)
	cards := []CardConfig{
		{CardID: 1, Key: "card_a", Name: "card_a", Rarity: "N", CardType: "material", Effects: []EffectConfig{{EffectType: "gain_resource"}}},
		{CardID: 1, Key: "card_b", Name: "card_b", Rarity: "N", CardType: "material", Effects: []EffectConfig{{EffectType: "gain_resource"}}},
	}
	orders := []OrderConfig{testOrderConfig()}
	levels := []LevelConfig{testLevelConfig()}

	_, err := NewGameData(cards, orders, levels, items)
	if err == nil {
		t.Fatalf("expected duplicate card_id error")
	}
}

func TestNewGameDataRejectsMissingCardReference(t *testing.T) {
	items := mustTestItemCatalog(t)
	cards := []CardConfig{testCardConfig()}
	orders := []OrderConfig{testOrderConfig()}
	level := testLevelConfig()
	level.FixedCards = []int64{999}

	_, err := NewGameData(cards, orders, []LevelConfig{level}, items)
	if err == nil {
		t.Fatalf("expected missing card reference error")
	}
}

func TestNewGameDataRejectsMissingRewardItem(t *testing.T) {
	items := mustTestItemCatalog(t)
	cards := []CardConfig{testCardConfig()}
	order := testOrderConfig()
	order.Rewards = []RewardConfig{{ItemID: 999, Count: 1}}
	levels := []LevelConfig{testLevelConfig()}

	_, err := NewGameData(cards, []OrderConfig{order}, levels, items)
	if err == nil {
		t.Fatalf("expected missing reward item error")
	}
}

func TestNewGameDataRejectsMissingCardUpgradeCostItem(t *testing.T) {
	items := mustTestItemCatalog(t)
	card := testCardConfig()
	card.UpgradeCosts = []CardUpgradeCostConfig{{
		TargetLevel: 2,
		Costs:       []CostConfig{{ItemID: 999, Count: 1}},
	}}

	_, err := NewGameData([]CardConfig{card}, []OrderConfig{testOrderConfig()}, []LevelConfig{testLevelConfig()}, items)
	if err == nil {
		t.Fatalf("expected missing card upgrade cost item error")
	}
}

func TestNewGameDataRejectsEmptyConfig(t *testing.T) {
	items := mustTestItemCatalog(t)

	_, err := NewGameData(nil, nil, nil, items)
	if err == nil {
		t.Fatalf("expected empty game data error")
	}
}

func TestLoadWorkshopData(t *testing.T) {
	items, err := LoadItemCatalog("../../configs/gamedata/items.json")
	if err != nil {
		t.Fatalf("LoadItemCatalog returned error: %v", err)
	}

	data, err := LoadWorkshopData("../../configs/gamedata/facilities.json", items)
	if err != nil {
		t.Fatalf("LoadWorkshopData returned error: %v", err)
	}
	if len(data.Facilities) != 6 {
		t.Fatalf("facilities = %d, want 6", len(data.Facilities))
	}
}

func TestNewWorkshopDataRejectsMissingUpgradeCostItem(t *testing.T) {
	items := mustTestItemCatalog(t)

	_, err := NewWorkshopData([]FacilityConfig{{
		FacilityID: "oven",
		Name:       "oven",
		MaxLevel:   2,
		Levels: []FacilityLevelConfig{
			{Level: 1},
			{Level: 2, UpgradeCosts: []CostConfig{{ItemID: 999, Count: 1}}},
		},
	}}, items)
	if err == nil {
		t.Fatalf("expected missing facility upgrade cost item error")
	}
}

func mustTestItemCatalog(t *testing.T) *Catalog {
	t.Helper()
	items, err := NewCatalog([]ItemConfig{
		{ItemID: ItemIDGold, Key: "gold", StorageType: StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: ItemIDBasicMaterial, Key: "basic_material", StorageType: StorageInventoryStack, Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog returned error: %v", err)
	}
	return items
}

func testCardConfig() CardConfig {
	return CardConfig{
		CardID:   1,
		Key:      "card",
		Name:     "card",
		Rarity:   "N",
		CardType: "material",
		Effects:  []EffectConfig{{EffectType: "gain_resource"}},
	}
}

func testOrderConfig() OrderConfig {
	return OrderConfig{
		OrderID:      1,
		Key:          "order",
		Name:         "order",
		OrderType:    "normal",
		Requirements: []ResourceAmount{{Resource: "bread", Count: 1}},
		Rewards:      []RewardConfig{{ItemID: ItemIDGold, Count: 1}},
	}
}

func testLevelConfig() LevelConfig {
	return LevelConfig{
		LevelID:            1,
		Name:               "level",
		Chapter:            1,
		TurnLimit:          1,
		ActionPointPerTurn: 1,
		OrderSlots:         1,
		InitialOrders:      1,
		Goal:               LevelGoal{GoalType: "complete_orders", Target: 1},
		FixedCards:         []int64{1},
		OrderPool:          []OrderPoolEntry{{OrderID: 1, Weight: 1}},
		FirstClearRewards:  []RewardConfig{{ItemID: ItemIDGold, Count: 1}},
		RepeatRewards:      []RewardConfig{{ItemID: ItemIDGold, Count: 1}},
	}
}
