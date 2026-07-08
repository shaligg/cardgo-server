package asset

import (
	"context"
	"errors"
	"testing"

	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakePlayerRepo struct {
	player     repo.Player
	lastReq    string
	lastReason string
	lastItemID int64
}

func (r *fakePlayerRepo) GetByUID(ctx context.Context, uid string) (repo.Player, error) {
	if r.player.UID == "" {
		r.player = repo.Player{UID: uid, Level: 1}
	}
	return r.player, nil
}

func (r *fakePlayerRepo) ChangeGold(ctx context.Context, uid string, delta int64, itemID int64, reason string, reqID string) (repo.Player, error) {
	if r.player.UID == "" {
		r.player = repo.Player{UID: uid, Level: 1}
	}
	if r.player.Gold+delta < 0 {
		return repo.Player{}, repo.ErrInsufficientGold
	}
	r.player.Gold += delta
	r.lastReq = reqID
	r.lastReason = reason
	r.lastItemID = itemID
	return r.player, nil
}

type fakeInventoryRepo struct {
	items map[int64]repo.InventoryItem
}

func (r *fakeInventoryRepo) GetInventory(ctx context.Context, uid string) ([]repo.InventoryItem, error) {
	out := make([]repo.InventoryItem, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeInventoryRepo) ChangeInventoryItem(ctx context.Context, uid string, itemID int64, delta int64, reason string, reqID string) (repo.InventoryItem, error) {
	if r.items == nil {
		r.items = map[int64]repo.InventoryItem{}
	}
	current := r.items[itemID]
	if current.UID == "" {
		current = repo.InventoryItem{UID: uid, ItemID: itemID}
	}
	if current.Count+delta < 0 {
		return repo.InventoryItem{}, repo.ErrInsufficientItem
	}
	current.Count += delta
	r.items[itemID] = current
	return current, nil
}

func newTestService(t *testing.T, players repo.PlayerRepository, inventory repo.InventoryRepository) Service {
	t.Helper()
	catalog, err := gamedata.NewCatalog([]gamedata.ItemConfig{
		{ItemID: gamedata.ItemIDGold, Key: "gold", StorageType: gamedata.StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: gamedata.ItemIDBasicMaterial, Key: "basic_material", StorageType: gamedata.StorageInventoryStack, Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog returned error: %v", err)
	}
	return Service{Items: catalog, Players: players, Inventory: inventory}
}

func TestGrantGold(t *testing.T) {
	players := &fakePlayerRepo{}
	svc := newTestService(t, players, nil)

	res, err := svc.Grant(context.Background(), "u1", []RewardItem{{ItemID: gamedata.ItemIDGold, Count: 100}}, "test.grant", "r1")
	if err != nil {
		t.Fatalf("Grant returned error: %v", err)
	}
	if res[0].Player == nil || res[0].Player.Gold != 100 {
		t.Fatalf("player = %+v, want gold 100", res[0].Player)
	}
	if players.lastReq != "r1" {
		t.Fatalf("reqID = %q, want raw request id", players.lastReq)
	}
	if players.lastReason != "test.grant" || players.lastItemID != gamedata.ItemIDGold {
		t.Fatalf("metadata = (%q, %d), want reason and item id", players.lastReason, players.lastItemID)
	}
}

func TestConsumeGold(t *testing.T) {
	players := &fakePlayerRepo{player: repo.Player{UID: "u1", Level: 1, Gold: 100}}
	svc := newTestService(t, players, nil)

	res, err := svc.Consume(context.Background(), "u1", []CostItem{{ItemID: gamedata.ItemIDGold, Count: 40}}, "test.consume", "r2")
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if res[0].Player == nil || res[0].Player.Gold != 60 {
		t.Fatalf("player = %+v, want gold 60", res[0].Player)
	}
}

func TestGrantInventoryStackItem(t *testing.T) {
	inventory := &fakeInventoryRepo{}
	svc := newTestService(t, nil, inventory)

	res, err := svc.Grant(context.Background(), "u1", []RewardItem{{ItemID: gamedata.ItemIDBasicMaterial, Count: 5}}, "test.grant", "r3")
	if err != nil {
		t.Fatalf("Grant inventory returned error: %v", err)
	}
	if res[0].Item == nil || res[0].Item.Count != 5 {
		t.Fatalf("item = %+v, want count 5", res[0].Item)
	}
}

func TestConsumeInventoryStackItem(t *testing.T) {
	inventory := &fakeInventoryRepo{items: map[int64]repo.InventoryItem{gamedata.ItemIDBasicMaterial: {UID: "u1", ItemID: gamedata.ItemIDBasicMaterial, Count: 5}}}
	svc := newTestService(t, nil, inventory)

	res, err := svc.Consume(context.Background(), "u1", []CostItem{{ItemID: gamedata.ItemIDBasicMaterial, Count: 2}}, "test.consume", "r4")
	if err != nil {
		t.Fatalf("Consume inventory returned error: %v", err)
	}
	if res[0].Item == nil || res[0].Item.Count != 3 {
		t.Fatalf("item = %+v, want count 3", res[0].Item)
	}
}

func TestRejectUnsupportedItem(t *testing.T) {
	svc := newTestService(t, &fakePlayerRepo{}, &fakeInventoryRepo{})

	_, err := svc.Grant(context.Background(), "u1", []RewardItem{{ItemID: 99999, Count: 1}}, "test", "r5")
	if !errors.Is(err, ErrUnsupportedItemID) {
		t.Fatalf("err = %v, want ErrUnsupportedItemID", err)
	}
}

func TestApplyRewardInTxMergesDuplicatedItems(t *testing.T) {
	svc, dbRepo, gdb := newRealAssetService(t)
	ctx := context.Background()

	err := gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := svc.ApplyRewardInTx(ctx, tx, "u1", []RewardItem{
			{ItemID: gamedata.ItemIDGold, Count: 1},
			{ItemID: gamedata.ItemIDGold, Count: 1},
		}, "test.merge_reward", "r6")
		return err
	})
	if err != nil {
		t.Fatalf("ApplyRewardInTx returned error: %v", err)
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 2 {
		t.Fatalf("gold = %d, want 2", player.Gold)
	}
}

func newRealAssetService(t *testing.T) (Service, *repo.DBPlayerRepository, *gorm.DB) {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	dbRepo := repo.NewDBPlayerRepository(gdb)
	if err := dbRepo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := newTestService(t, dbRepo, dbRepo)
	svc.Tx = idb.NewTxManager(gdb)
	svc.TxPlayers = dbRepo
	svc.TxInventory = dbRepo
	return svc, dbRepo, gdb
}

func TestGrantBatchUsesOwnTransaction(t *testing.T) {
	svc, dbRepo, _ := newRealAssetService(t)
	ctx := context.Background()

	res, err := svc.Grant(ctx, "u1", []RewardItem{
		{ItemID: gamedata.ItemIDGold, Count: 10},
		{ItemID: gamedata.ItemIDGold, Count: 5},
		{ItemID: gamedata.ItemIDBasicMaterial, Count: 3},
	}, "test.grant_batch", "batch-r1")
	if err != nil {
		t.Fatalf("Grant batch returned error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("result len = %d, want 2", len(res))
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 15 {
		t.Fatalf("gold = %d, want 15", player.Gold)
	}
	items, err := dbRepo.GetInventory(ctx, "u1")
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != gamedata.ItemIDBasicMaterial || items[0].Count != 3 {
		t.Fatalf("items = %+v, want basic material count 3", items)
	}
}

func TestApplyRewardInTxCommitsWithOuterTransaction(t *testing.T) {
	svc, dbRepo, gdb := newRealAssetService(t)
	ctx := context.Background()

	err := gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := svc.ApplyRewardInTx(ctx, tx, "u1", []RewardItem{
			{ItemID: gamedata.ItemIDGold, Count: 100},
			{ItemID: gamedata.ItemIDBasicMaterial, Count: 5},
		}, "test.tx_reward", "tx-r1")
		return err
	})
	if err != nil {
		t.Fatalf("ApplyRewardInTx transaction returned error: %v", err)
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 100 {
		t.Fatalf("gold = %d, want 100", player.Gold)
	}
	items, err := dbRepo.GetInventory(ctx, "u1")
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != gamedata.ItemIDBasicMaterial || items[0].Count != 5 {
		t.Fatalf("items = %+v, want basic material count 5", items)
	}
}

func TestApplyRewardInTxRollsBackWithOuterTransaction(t *testing.T) {
	svc, dbRepo, gdb := newRealAssetService(t)
	ctx := context.Background()
	wantErr := errors.New("rollback")

	err := gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := svc.ApplyRewardInTx(ctx, tx, "u1", []RewardItem{{ItemID: gamedata.ItemIDGold, Count: 100}}, "test.tx_reward", "tx-r2"); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 0 {
		t.Fatalf("gold = %d, want rollback to 0", player.Gold)
	}
}

func TestApplyCostInTxConsumesMultipleAssets(t *testing.T) {
	svc, dbRepo, gdb := newRealAssetService(t)
	ctx := context.Background()
	if _, err := dbRepo.ChangeGold(ctx, "u1", 100, gamedata.ItemIDGold, "test.prepare", "prepare-gold"); err != nil {
		t.Fatalf("prepare gold: %v", err)
	}
	if _, err := dbRepo.ChangeInventoryItem(ctx, "u1", gamedata.ItemIDBasicMaterial, 5, "test.prepare", "prepare-item"); err != nil {
		t.Fatalf("prepare item: %v", err)
	}

	err := gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := svc.ApplyCostInTx(ctx, tx, "u1", []CostItem{
			{ItemID: gamedata.ItemIDGold, Count: 40},
			{ItemID: gamedata.ItemIDBasicMaterial, Count: 2},
		}, "test.tx_cost", "tx-c1")
		return err
	})
	if err != nil {
		t.Fatalf("ApplyCostInTx transaction returned error: %v", err)
	}

	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 60 {
		t.Fatalf("gold = %d, want 60", player.Gold)
	}
	items, err := dbRepo.GetInventory(ctx, "u1")
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if len(items) != 1 || items[0].Count != 3 {
		t.Fatalf("items = %+v, want material count 3", items)
	}
}
