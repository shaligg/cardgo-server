package workshop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/game/asset"
	"github.com/bigfish/go_orm_1/internal/gamedata"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestWorkshopService(t *testing.T) (Service, *repo.DBPlayerRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	dbRepo := repo.NewDBPlayerRepository(db)
	if err := dbRepo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	items, err := gamedata.NewCatalog([]gamedata.ItemConfig{
		{ItemID: gamedata.ItemIDGold, Key: "gold", StorageType: gamedata.StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: gamedata.ItemIDBasicMaterial, Key: "basic_material", StorageType: gamedata.StorageInventoryStack, Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	workshopData, err := gamedata.NewWorkshopData([]gamedata.FacilityConfig{{
		FacilityID: "oven",
		Name:       "烤炉",
		MaxLevel:   5,
		Levels: []gamedata.FacilityLevelConfig{
			{Level: 1},
			{Level: 2, UpgradeCosts: []gamedata.CostConfig{{ItemID: gamedata.ItemIDGold, Count: 100}, {ItemID: gamedata.ItemIDBasicMaterial, Count: 2}}},
			{Level: 3, UpgradeCosts: []gamedata.CostConfig{{ItemID: gamedata.ItemIDGold, Count: 300}, {ItemID: gamedata.ItemIDBasicMaterial, Count: 4}}},
			{Level: 4, UpgradeCosts: []gamedata.CostConfig{{ItemID: gamedata.ItemIDGold, Count: 800}, {ItemID: gamedata.ItemIDBasicMaterial, Count: 8}}},
			{Level: 5, UpgradeCosts: []gamedata.CostConfig{{ItemID: gamedata.ItemIDGold, Count: 1600}, {ItemID: gamedata.ItemIDBasicMaterial, Count: 16}}},
		},
	}}, items)
	if err != nil {
		t.Fatalf("NewWorkshopData: %v", err)
	}
	assets := asset.Service{Items: items, Players: dbRepo, Inventory: dbRepo, TxPlayers: dbRepo, TxInventory: dbRepo}
	return Service{Repo: dbRepo, Assets: assets, Tx: idb.NewTxManager(db), Players: dbRepo, Data: workshopData}, dbRepo, db
}

func TestGetOverviewCreatesDefaultWorkshop(t *testing.T) {
	svc, _, _ := newTestWorkshopService(t)
	overview, err := svc.GetOverview(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetOverview returned error: %v", err)
	}
	if overview.Workshop.UID != "u1" {
		t.Fatalf("workshop uid = %q, want u1", overview.Workshop.UID)
	}
	if overview.Workshop.Level != 1 {
		t.Fatalf("workshop level = %d, want 1", overview.Workshop.Level)
	}
	if overview.Workshop.ActiveThemeID != "default" {
		t.Fatalf("active theme = %q, want default", overview.Workshop.ActiveThemeID)
	}
	if overview.Workshop.LastOfflineRewardAt <= 0 {
		t.Fatalf("last_offline_reward_at = %d, want positive unix timestamp", overview.Workshop.LastOfflineRewardAt)
	}
	if len(overview.Facilities) != 0 {
		t.Fatalf("facilities = %+v, want empty before the player upgrades a facility", overview.Facilities)
	}
}

func TestUpgradeFacilityConsumesGoldAndLevelsUp(t *testing.T) {
	svc, dbRepo, _ := newTestWorkshopService(t)
	ctx := context.Background()
	if _, err := dbRepo.ChangeGold(ctx, "u1", 200, gamedata.ItemIDGold, "test.grant", "gold-r1"); err != nil {
		t.Fatalf("grant gold: %v", err)
	}
	if _, err := dbRepo.ChangeInventoryItem(ctx, "u1", gamedata.ItemIDBasicMaterial, 5, "test.grant", "mat-r1"); err != nil {
		t.Fatalf("grant material: %v", err)
	}

	result, err := svc.UpgradeFacility(ctx, "u1", "oven", "facility-r1")
	if err != nil {
		t.Fatalf("UpgradeFacility returned error: %v", err)
	}
	if result.Facility.Level != 2 || result.OldLevel != 1 || result.NewLevel != 2 || result.GoldCost != 100 || len(result.Costs) != 2 {
		t.Fatalf("upgrade result = %+v, want level 1 -> 2 cost gold 100 and material 2", result)
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
	if len(items) != 1 || items[0].Count != 3 {
		t.Fatalf("items = %+v, want material count 3", items)
	}

}

func TestUpgradeFacilityRejectsUnknownFacility(t *testing.T) {
	svc, _, _ := newTestWorkshopService(t)
	_, err := svc.UpgradeFacility(context.Background(), "u1", "unknown", "facility-r1")
	if !errors.Is(err, ErrFacilityNotFound) {
		t.Fatalf("err = %v, want ErrFacilityNotFound", err)
	}
}

func TestClaimOfflineRewardGrantsGoldAndMaterial(t *testing.T) {
	svc, dbRepo, db := newTestWorkshopService(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	svc.Now = func() time.Time { return now }

	if _, err := svc.GetOverview(ctx, "u1"); err != nil {
		t.Fatalf("GetOverview: %v", err)
	}
	if err := db.Model(&model.PlayerWorkshop{}).
		Where("uid = ?", "u1").
		Update("last_offline_reward_at", now.Add(-2*time.Hour)).Error; err != nil {
		t.Fatalf("set last_offline_reward_at: %v", err)
	}

	overview, err := svc.GetOverview(ctx, "u1")
	if err != nil {
		t.Fatalf("GetOverview after offline: %v", err)
	}
	if overview.OfflineRewardPreview.OfflineSeconds != 7200 || overview.OfflineRewardPreview.Gold != 40 || overview.OfflineRewardPreview.BasicMaterial != 2 {
		t.Fatalf("preview = %+v, want 7200 seconds, 40 gold and 2 material", overview.OfflineRewardPreview)
	}

	result, err := svc.ClaimOfflineReward(ctx, "u1", "offline-r1")
	if err != nil {
		t.Fatalf("ClaimOfflineReward returned error: %v", err)
	}
	if result.EffectiveSeconds != 7200 || result.Gold != 40 || result.Preview.BasicMaterial != 2 || len(result.Rewards) != 2 {
		t.Fatalf("claim result = %+v, want 7200 seconds, 40 gold and 2 material", result)
	}
	player, err := dbRepo.GetByUID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if player.Gold != 40 {
		t.Fatalf("gold = %d, want 40", player.Gold)
	}
	items, err := dbRepo.GetInventory(ctx, "u1")
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != gamedata.ItemIDBasicMaterial || items[0].Count != 2 {
		t.Fatalf("items = %+v, want basic material count 2", items)
	}

}
