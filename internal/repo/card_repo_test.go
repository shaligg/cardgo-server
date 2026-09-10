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
	if err := cardRepo.EnsureDefaultCards(ctx, "u1", []int64{10001}); err != nil {
		t.Fatalf("EnsureDefaultCards: %v", err)
	}

	var card PlayerCard
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		card, err = cardRepo.GetCardInTx(ctx, tx, "u1", 10001)
		if err != nil {
			return err
		}
		card.Level++
		return cardRepo.UpdateCardInTx(ctx, tx, card)
	})
	if err != nil {
		t.Fatalf("update card transaction returned error: %v", err)
	}
	if card.Level != 2 {
		t.Fatalf("card level = %d, want 2", card.Level)
	}
	cards, err := cardRepo.GetCards(ctx, "u1")
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if len(cards) != 1 || cards[0].Level != 2 {
		t.Fatalf("stored cards = %+v, want level 2", cards)
	}
}
