package repo

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func TestCardRepositoryReadsAndUpdatesCalculatedCardInTx(t *testing.T) {
	db := testdb.OpenGame(t)
	cardRepo := NewDBCardRepository(db)
	ctx := context.Background()
	card := PlayerCard{UID: "u1", CardID: 10001, Level: 2, Exp: 4, Count: 3}
	if err := cardRepo.CreateCardsIfAbsent(ctx, []PlayerCard{card}); err != nil {
		t.Fatalf("CreateCardsIfAbsent: %v", err)
	}

	var updated PlayerCard
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		updated, err = cardRepo.GetCardInTx(ctx, tx, "u1", 10001)
		if err != nil {
			return err
		}
		updated.Level++
		return cardRepo.UpdateCardInTx(ctx, tx, updated)
	})
	if err != nil {
		t.Fatalf("update card transaction returned error: %v", err)
	}
	if updated.Level != 3 {
		t.Fatalf("card level = %d, want 3", updated.Level)
	}
	cards, err := cardRepo.GetCards(ctx, "u1")
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if len(cards) != 1 || cards[0].Level != 3 || cards[0].Exp != 4 || cards[0].Count != 3 {
		t.Fatalf("stored cards = %+v, want calculated card values", cards)
	}
}
