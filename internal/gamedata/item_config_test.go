package gamedata

import "testing"

func TestNewCatalog(t *testing.T) {
	catalog, err := NewCatalog([]ItemConfig{
		{ItemID: ItemIDGold, Key: "gold", StorageType: StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: ItemIDBasicMaterial, Key: "basic_material", StorageType: StorageInventoryStack, Stackable: true},
	})
	if err != nil {
		t.Fatalf("NewCatalog returned error: %v", err)
	}
	gold, ok := catalog.GetItem(ItemIDGold)
	if !ok || gold.StorageType != StoragePlayerField {
		t.Fatalf("gold config = %+v, ok=%v", gold, ok)
	}
}

func TestNewCatalogRejectsDuplicateItemID(t *testing.T) {
	_, err := NewCatalog([]ItemConfig{
		{ItemID: 1, Key: "gold", StorageType: StoragePlayerField, StorageKey: "gold", Stackable: true},
		{ItemID: 1, Key: "gold2", StorageType: StoragePlayerField, StorageKey: "gold", Stackable: true},
	})
	if err == nil {
		t.Fatalf("expected duplicate item_id error")
	}
}

func TestNewCatalogRejectsInvalidStorage(t *testing.T) {
	_, err := NewCatalog([]ItemConfig{{ItemID: 10001, Key: "basic_material", StorageType: StorageInventoryStack}})
	if err == nil {
		t.Fatalf("expected invalid storage error")
	}
}
