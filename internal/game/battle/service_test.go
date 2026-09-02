package battle

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

type fakePlayerRepo struct {
	player        repo.Player
	grantCalls    int
	failNextGrant bool
}

func (r *fakePlayerRepo) GetByUID(ctx context.Context, uid string) (repo.Player, error) {
	_ = ctx
	if r.player.UID == "" {
		r.player = repo.Player{UID: uid, Level: 1}
	}
	return r.player, nil
}

func (r *fakePlayerRepo) ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (repo.Player, error) {
	_ = ctx
	_ = itemID
	_ = reason
	_ = reqID
	if r.player.UID == "" {
		r.player = repo.Player{UID: uid, Level: 1}
	}
	r.player.Gold += delta
	r.grantCalls++
	return r.player, nil
}

func (r *fakePlayerRepo) ChangeGoldInTx(ctx context.Context, tx *gorm.DB, uid string, delta int64, itemID int64, reason string, reqID string) (repo.Player, error) {
	_ = tx
	if r.failNextGrant {
		r.failNextGrant = false
		return repo.Player{}, errors.New("forced grant failure")
	}
	return r.ChangeGold(ctx, uid, delta, itemID, reason, reqID)
}

type fakeInventoryRepo struct {
	items      map[int64]repo.InventoryItem
	grantCalls int
}

func (r *fakeInventoryRepo) GetInventory(ctx context.Context, uid string) ([]repo.InventoryItem, error) {
	_ = ctx
	out := make([]repo.InventoryItem, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeInventoryRepo) ChangeInventoryItem(ctx context.Context, uid string, itemID int64, delta int64, reason string, reqID string) (repo.InventoryItem, error) {
	_ = ctx
	_ = reason
	_ = reqID
	if r.items == nil {
		r.items = map[int64]repo.InventoryItem{}
	}
	item := r.items[itemID]
	if item.UID == "" {
		item = repo.InventoryItem{UID: uid, ItemID: itemID}
	}
	item.Count += delta
	r.items[itemID] = item
	r.grantCalls++
	return item, nil
}

func (r *fakeInventoryRepo) ChangeInventoryItemInTx(ctx context.Context, tx *gorm.DB, uid string, itemID int64, delta int64, reason string, reqID string) (repo.InventoryItem, error) {
	_ = tx
	return r.ChangeInventoryItem(ctx, uid, itemID, delta, reason, reqID)
}

func TestLevelFlowSettleGrantsRewardsOnce(t *testing.T) {
	players := &fakePlayerRepo{}
	inventory := &fakeInventoryRepo{}
	svc := newTestBattleService(t, players, inventory)

	session, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("StartLevel returned error: %v", err)
	}
	if session.LevelID != 1 || len(session.ActiveOrders) != 1 {
		t.Fatalf("unexpected session: %+v", session)
	}

	state, err := svc.PlayCard(context.Background(), "u1", session.SessionID, 10001, "play-1")
	if err != nil {
		t.Fatalf("PlayCard returned error: %v", err)
	}
	if state.Session.CompletedOrders != 1 {
		t.Fatalf("completed_orders = %d, want 1", state.Session.CompletedOrders)
	}

	result, err := svc.SettleLevel(context.Background(), "u1", session.SessionID, "settle-1")
	if err != nil {
		t.Fatalf("SettleLevel returned error: %v", err)
	}
	if !result.OK || result.CompletedOrders != 1 {
		t.Fatalf("unexpected settle result: %+v", result)
	}
	if result.Player == nil || result.Player.Gold != 28 {
		t.Fatalf("settle player = %+v, want gold 28", result.Player)
	}
	if players.player.Gold != 28 {
		t.Fatalf("gold = %d, want 28", players.player.Gold)
	}
	if inventory.items[gamedata.ItemIDBasicMaterial].Count != 2 {
		t.Fatalf("basic_material = %d, want 2", inventory.items[gamedata.ItemIDBasicMaterial].Count)
	}

	_, err = svc.SettleLevel(context.Background(), "u1", session.SessionID, "settle-2")
	if err != nil {
		t.Fatalf("repeated SettleLevel returned error: %v", err)
	}
	if players.grantCalls != 1 || inventory.grantCalls != 1 {
		t.Fatalf("repeated settle should not grant again, player_calls=%d inventory_calls=%d", players.grantCalls, inventory.grantCalls)
	}
}

func TestStartLevelRequiresReqID(t *testing.T) {
	svc := newTestBattleService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})

	if _, err := svc.StartLevel(context.Background(), "u1", 1, ""); !errors.Is(err, ErrInvalidReqID) {
		t.Fatalf("StartLevel error = %v, want ErrInvalidReqID", err)
	}
}

func TestPlayCardRequiresReqID(t *testing.T) {
	svc := newTestBattleService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})
	session, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("StartLevel: %v", err)
	}

	if _, err := svc.PlayCard(context.Background(), "u1", session.SessionID, 10001, ""); !errors.Is(err, ErrInvalidReqID) {
		t.Fatalf("PlayCard error = %v, want ErrInvalidReqID", err)
	}
}

func TestPlayCardFailureDoesNotReserveReqID(t *testing.T) {
	svc := newTestBattleService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})
	session, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("StartLevel: %v", err)
	}
	if _, err := svc.PlayCard(context.Background(), "u1", session.SessionID, 10002, "play-1"); !errors.Is(err, ErrInsufficientResource) {
		t.Fatalf("failed PlayCard error = %v, want ErrInsufficientResource", err)
	}
	if current := svc.sessions[session.SessionID].state.Resources["bread"]; current != 0 {
		t.Fatalf("failed PlayCard left partial state, bread = %d", current)
	}

	if _, err := svc.PlayCard(context.Background(), "u1", session.SessionID, 10001, "play-1"); err != nil {
		t.Fatalf("retry after failed PlayCard: %v", err)
	}
}

func TestSettleLevelRejectsIncompleteLevel(t *testing.T) {
	svc := newTestBattleService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})
	session, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("StartLevel returned error: %v", err)
	}

	_, err = svc.SettleLevel(context.Background(), "u1", session.SessionID, "settle-1")
	if err == nil {
		t.Fatalf("expected incomplete level error")
	}
}

func TestSettleLevelKeepsSessionUnsettledWhenRewardTransactionFails(t *testing.T) {
	players := &fakePlayerRepo{failNextGrant: true}
	svc := newTestBattleService(t, players, &fakeInventoryRepo{})
	session, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("StartLevel returned error: %v", err)
	}
	if _, err := svc.PlayCard(context.Background(), "u1", session.SessionID, 10001, "play-1"); err != nil {
		t.Fatalf("PlayCard returned error: %v", err)
	}

	if _, err := svc.SettleLevel(context.Background(), "u1", session.SessionID, "settle-1"); err == nil {
		t.Fatal("expected reward transaction error")
	}
	if rs := svc.sessions[session.SessionID]; rs.state.Settled || rs.settleResult != nil {
		t.Fatalf("failed transaction marked session settled: %+v", rs.state)
	}

	result, err := svc.SettleLevel(context.Background(), "u1", session.SessionID, "settle-1")
	if err != nil {
		t.Fatalf("retry SettleLevel returned error: %v", err)
	}
	if !result.OK || players.player.Gold != 28 {
		t.Fatalf("unexpected retry result=%+v gold=%d", result, players.player.Gold)
	}
}

func TestDeletePlayerRuntimeRemovesOnlyTargetPlayer(t *testing.T) {
	svc := newTestBattleService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})
	first, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("start u1: %v", err)
	}
	second, err := svc.StartLevel(context.Background(), "u2", 1, "start-2")
	if err != nil {
		t.Fatalf("start u2: %v", err)
	}

	if deleted := svc.DeletePlayerRuntime("u1"); deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := svc.PlayCard(context.Background(), "u1", first.SessionID, 10001, "play-1"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("u1 play error = %v, want ErrSessionNotFound", err)
	}
	if _, err := svc.PlayCard(context.Background(), "u2", second.SessionID, 10001, "play-2"); err != nil {
		t.Fatalf("u2 runtime was removed: %v", err)
	}
	restarted, err := svc.StartLevel(context.Background(), "u1", 1, "start-1")
	if err != nil {
		t.Fatalf("restart u1 after runtime deletion: %v", err)
	}
	if restarted.SessionID == first.SessionID {
		t.Fatal("new runtime reused the deleted session id")
	}
}

func newTestBattleService(t *testing.T, players repo.PlayerRepository, inventory repo.InventoryRepository) *Service {
	t.Helper()
	gdb := testdb.OpenGame(t)
	items, err := gamedata.NewCatalog([]gamedata.ItemConfig{
		{ItemID: gamedata.ItemIDGold, Key: "gold", StorageType: gamedata.StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: gamedata.ItemIDBasicMaterial, Key: "basic_material", StorageType: gamedata.StorageInventoryStack, Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog returned error: %v", err)
	}
	data, err := gamedata.NewGameData(
		[]gamedata.CardConfig{
			{
				CardID:   10001,
				Key:      "wheat_bag",
				Name:     "小麦袋",
				Rarity:   "N",
				CardType: "material",
				Cost:     1,
				Effects:  []gamedata.EffectConfig{{EffectType: "gain_resource", Resource: "bread", Value: 2}},
			},
			{
				CardID:   10002,
				Key:      "failed_combo",
				Name:     "失败组合",
				Rarity:   "N",
				CardType: "material",
				Cost:     1,
				Effects: []gamedata.EffectConfig{
					{EffectType: "gain_resource", Resource: "bread", Value: 1},
					{EffectType: "convert_resource", Resource: "wood", Value: 1, ToResource: "bread", ToValue: 1},
				},
			},
		},
		[]gamedata.OrderConfig{{
			OrderID:      20001,
			Key:          "white_bread",
			Name:         "白面包",
			OrderType:    "normal",
			Requirements: []gamedata.ResourceAmount{{Resource: "bread", Count: 2}},
			Rewards:      []gamedata.RewardConfig{{ItemID: gamedata.ItemIDGold, Count: 8}},
		}},
		[]gamedata.LevelConfig{{
			LevelID:            1,
			Name:               "第一份订单",
			Chapter:            1,
			TurnLimit:          3,
			ActionPointPerTurn: 3,
			OrderSlots:         1,
			InitialOrders:      1,
			Goal:               gamedata.LevelGoal{GoalType: "complete_orders", Target: 1},
			FixedCards:         []int64{10001, 10002},
			OrderPool:          []gamedata.OrderPoolEntry{{OrderID: 20001, Weight: 100}},
			FirstClearRewards: []gamedata.RewardConfig{
				{ItemID: gamedata.ItemIDGold, Count: 20},
				{ItemID: gamedata.ItemIDBasicMaterial, Count: 2},
			},
			RepeatRewards: []gamedata.RewardConfig{{ItemID: gamedata.ItemIDGold, Count: 8}},
		}},
		items,
	)
	if err != nil {
		t.Fatalf("NewGameData returned error: %v", err)
	}
	return &Service{
		Data: data,
		Tx:   idb.NewTxManager(gdb),
		Assets: asset.Service{
			Items:       items,
			Players:     players,
			Inventory:   inventory,
			TxPlayers:   players.(repo.TxPlayerRepository),
			TxInventory: inventory.(repo.TxInventoryRepository),
		},
	}
}
