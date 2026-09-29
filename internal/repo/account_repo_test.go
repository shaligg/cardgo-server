package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func TestAccountPersistenceAndConflictRollback(t *testing.T) {
	db := testdb.Open(t, &model.Account{}, &model.AccountIdentity{})
	ctx := context.Background()
	r := NewDBAccountRepository(db)
	if err := r.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	create := func(uid string) error {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := r.CreateAccountInTx(ctx, tx, Account{UID: uid, Status: "active"}); err != nil {
				return err
			}
			return r.CreateIdentityInTx(ctx, tx, AccountIdentity{Provider: "local", Subject: "user", UID: uid, PasswordHash: "hash"})
		})
	}
	if err := create("first"); err != nil {
		t.Fatal(err)
	}
	if err := create("second"); !errors.Is(err, ErrAccountIdentityExists) {
		t.Fatalf("duplicate error %v", err)
	}
	var count int64
	db.Model(&model.Account{}).Count(&count)
	if count != 1 {
		t.Fatalf("orphan account count=%d", count)
	}
	i, err := r.Identity(ctx, "local", "user")
	if err != nil || i.UID != "first" {
		t.Fatalf("identity %+v %v", i, err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	for _, want := range []Account{{TokenHash: "hash", ExpiresAt: &now}, {}} {
		err = db.Transaction(func(tx *gorm.DB) error {
			a, err := r.AccountInTx(ctx, tx, "first")
			if err != nil {
				return err
			}
			a.TokenHash, a.ExpiresAt = want.TokenHash, want.ExpiresAt
			return r.SaveAccountInTx(ctx, tx, a)
		})
		if err != nil {
			t.Fatal(err)
		}
		var got model.Account
		if err := db.Where("uid = ?", "first").Take(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.TokenHash != want.TokenHash {
			t.Fatal("current token hash not persisted")
		}
		if want.ExpiresAt == nil {
			if got.ExpiresAt != nil {
				t.Fatal("logout expiry not cleared")
			}
		} else if got.ExpiresAt == nil || !got.ExpiresAt.Equal(*want.ExpiresAt) {
			t.Fatal("current expiry not persisted")
		}
	}
	// 启动探测拒绝缺少任一新字段的旧两表结构。
	for _, column := range []string{"device_id", "guest_device_id", "token_hash"} {
		if err := db.Migrator().DropColumn(&model.Account{}, column); err != nil {
			t.Fatal(err)
		}
		if err := r.CheckSchema(ctx); err == nil {
			t.Fatal("schema check accepted missing column", column)
		}
		if err := db.AutoMigrate(&model.Account{}); err != nil {
			t.Fatal(err)
		}
	}
}
