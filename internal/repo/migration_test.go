package repo

import (
	"testing"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func TestMigrateIncludesGameplayTables(t *testing.T) {
	db := testdb.OpenGame(t)
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	for name, table := range map[string]interface{}{
		"player_workshops":        &model.PlayerWorkshop{},
		"player_facilities":       &model.PlayerFacility{},
		"player_level_progresses": &model.PlayerLevelProgress{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("%s table was not migrated", name)
		}
	}
}

func TestMigrateRejectsNilDatabase(t *testing.T) {
	if err := Migrate(nil); err == nil {
		t.Fatal("Migrate(nil) returned nil error")
	}
}
