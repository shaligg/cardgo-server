package account

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
	"gorm.io/gorm"
)

func testService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := testdb.Open(t, &model.Account{}, &model.AccountIdentity{})
	s, err := NewService(repo.NewDBAccountRepository(db), idb.NewTxManager(db), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s, db
}
func TestConcurrentRegistration(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan string, 4)
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uid, err := s.Register(ctx, " Same_User ", "correct-password")
			results <- uid
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	winner := ""
	for uid := range results {
		if uid != "" {
			if winner != "" {
				t.Fatal("multiple accounts created")
			}
			winner = uid
		}
	}
	if winner == "" {
		t.Fatal("no registration won")
	}
	for err := range failures {
		if err != nil && !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
	var count int64
	db.Model(&model.Account{}).Count(&count)
	if count != 1 {
		t.Fatalf("accounts=%d", count)
	}
	uid, err := s.verifyPassword(ctx, "same_user", "correct-password")
	if err != nil || uid != winner {
		t.Fatalf("identity changed: %s %v", uid, err)
	}
	for _, name := range []string{"same_user", "missing_user"} {
		if _, err := s.verifyPassword(ctx, name, "wrong-password"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("credential error %v", err)
		}
	}
}

// accountRow 直接检查数据库中的当前凭证，避免只验证服务返回值。
func accountRow(t *testing.T, db *gorm.DB, uid string) model.Account {
	t.Helper()
	var row model.Account
	if err := db.Where("uid = ?", uid).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestSessionIdleExpiryAndReplacement(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Second)
	now := start
	s.now = func() time.Time { return now }
	uid, err := s.Register(ctx, "player", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, "player", "correct-password")
	if err != nil || p.UID != uid || p.SessionExpireAt != start.Add(30*24*time.Hour).Unix() {
		t.Fatal("login identity or deadline", err)
	}
	initial := accountRow(t, db, p.UID)
	if initial.TokenHash != tokenHash(p.SessionToken) || initial.TokenHash == p.SessionToken {
		t.Fatal("plaintext token persisted")
	}
	now = start.Add(24 * time.Hour)
	forged, err := newToken(initial.UID)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{forged, "a" + p.SessionToken[1:], "r" + p.SessionToken[1:], "invalid"} {
		if _, _, err := s.Authenticate(ctx, token); err != ErrInvalid {
			t.Fatal("invalid credential accepted", err)
		}
		if err := s.Logout(ctx, token); err != nil {
			t.Fatal("unknown logout not idempotent", err)
		}
	}
	unchanged := accountRow(t, db, p.UID)
	if !unchanged.ExpiresAt.Equal(*initial.ExpiresAt) || unchanged.TokenHash != initial.TokenHash {
		t.Fatal("forged token changed victim session")
	}
	// 第 29 天和第 58 天都使用同一令牌，跨过最初 30 天期限。
	for _, day := range []int{29, 58} {
		now = start.Add(time.Duration(day) * 24 * time.Hour)
		got, expiry, err := s.Authenticate(ctx, p.SessionToken)
		if err != nil || got != uid || expiry != now.Add(30*24*time.Hour).Unix() {
			t.Fatal("idle renewal", day, err)
		}
		row := accountRow(t, db, p.UID)
		if row.TokenHash != initial.TokenHash || row.ExpiresAt.Unix() != expiry {
			t.Fatal("renewal rotated token or failed to persist deadline")
		}
	}
	// 新设备重新验证密码后，旧 Token 失效；旧设备退出不能撤销新会话。
	old := p
	p, err = s.Login(ctx, "player", "correct-password")
	if err != nil || p.UID != uid || p.SessionToken == old.SessionToken {
		t.Fatal("new login identity or token", err)
	}
	if _, _, err := s.Authenticate(ctx, old.SessionToken); err != ErrInvalid {
		t.Fatal("replaced token survived new login", err)
	}
	if err := s.Logout(ctx, old.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, p.SessionToken); err != nil {
		t.Fatal("old device logout affected current session", err)
	}
	before := accountRow(t, db, p.UID)
	now = *before.ExpiresAt
	if _, _, err := s.Authenticate(ctx, p.SessionToken); err != ErrExpired {
		t.Fatal("exact expiry boundary accepted", err)
	}
	if row := accountRow(t, db, p.UID); !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("expired session revived")
	}
	again, err := s.Login(ctx, "player", "correct-password")
	if err != nil || again.UID != uid || again.SessionToken == p.SessionToken {
		t.Fatal("reauthentication did not recover same UID", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Logout(ctx, again.SessionToken); err != nil {
			t.Fatal("logout not idempotent", err)
		}
	}
	if _, _, err := s.Authenticate(ctx, again.SessionToken); err != ErrInvalid {
		t.Fatal("revoked session accepted", err)
	}
}

func TestInactiveAccountCannotLoginOrRenew(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	uid, err := s.Register(ctx, "blocked", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"banned", "deleted"} {
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("status", "active").Error; err != nil {
			t.Fatal(err)
		}
		p, err := s.Login(ctx, "blocked", "correct-password")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		now = now.Add(24 * time.Hour)
		if _, err := s.Login(ctx, "blocked", "correct-password"); err != ErrInvalid {
			t.Fatal("inactive login", status, err)
		}
		if _, _, err := s.Authenticate(ctx, p.SessionToken); err != ErrInvalid {
			t.Fatal("inactive session accepted", status, err)
		}
		if row := accountRow(t, db, p.UID); row.ExpiresAt.Unix() != p.SessionExpireAt {
			t.Fatal("inactive account renewed")
		}
	}
}

func TestConcurrentUseRetryRestartAndLogout(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "concurrent", "correct-password"); err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, "concurrent", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan error, 8)
	for i := 0; i < cap(ch); i++ {
		go func() { _, _, err := s.Authenticate(ctx, p.SessionToken); ch <- err }()
	}
	for i := 0; i < cap(ch); i++ {
		if err := <-ch; err != nil {
			t.Fatal("concurrent use rejected", err)
		}
	}
	// 丢弃前次响应，再创建 Service 模拟重启；原令牌仍可恢复同一 UID。
	restarted, err := NewService(repo.NewDBAccountRepository(db), idb.NewTxManager(db), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if uid, _, err := restarted.Authenticate(ctx, p.SessionToken); err != nil || uid != p.UID {
		t.Fatal("retry after restart failed", err)
	}
	// 两个设备并发密码登录都可返回，只有最后提交的会话能继续使用。
	logins := make(chan LoginResult, 2)
	for i := 0; i < cap(logins); i++ {
		go func() {
			result, err := s.Login(ctx, "concurrent", "correct-password")
			logins <- result
			ch <- err
		}()
	}
	for i := 0; i < cap(logins); i++ {
		if err := <-ch; err != nil {
			t.Fatal("concurrent login failed", err)
		}
	}
	if _, _, err := restarted.Authenticate(ctx, p.SessionToken); err != ErrInvalid {
		t.Fatal("previous login survived replacement", err)
	}
	valid := 0
	for i := 0; i < cap(logins); i++ {
		candidate := <-logins
		if _, _, err := s.Authenticate(ctx, candidate.SessionToken); err == nil {
			valid++
			p = candidate
		} else if err != ErrInvalid {
			t.Fatal(err)
		}
	}
	if valid != 1 {
		t.Fatalf("current sessions=%d, want 1", valid)
	}
	go func() { ch <- s.Logout(ctx, p.SessionToken) }()
	go func() { _, _, err := restarted.Authenticate(ctx, p.SessionToken); ch <- err }()
	for i := 0; i < 2; i++ {
		if err := <-ch; err != nil && err != ErrInvalid {
			t.Fatal("concurrent logout/use failed", err)
		}
	}
	if _, _, err := s.Authenticate(ctx, p.SessionToken); err != ErrInvalid {
		t.Fatal("concurrent renewal revived logged-out session", err)
	}
	row := accountRow(t, db, p.UID)
	if row.TokenHash != "" || row.ExpiresAt != nil {
		t.Fatal("logout did not clear current credentials")
	}
}

func TestSessionWriteFailuresRollBack(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	if _, err := s.Register(ctx, "rollback", "correct-password"); err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, "rollback", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	before := accountRow(t, db, p.UID)
	now = now.Add(24 * time.Hour)
	if _, err := s.Login(ctx, "rollback", "wrong-password"); err != ErrInvalid {
		t.Fatal("wrong password accepted", err)
	}
	for _, operation := range []string{"renew", "login", "logout"} {
		failure := errors.New("injected session persistence failure")
		name := "account_test_session_failure"
		// UPDATE 已执行后注入错误，复用同一用例检查续期、登录覆盖和退出清空的整体回滚。
		if err := db.Callback().Update().After("gorm:update").Register(name, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Account" {
				tx.AddError(failure)
			}
		}); err != nil {
			t.Fatal(err)
		}
		var uid string
		var expiry int64
		if operation == "renew" {
			uid, expiry, err = s.Authenticate(ctx, p.SessionToken)
		} else if operation == "login" {
			var result LoginResult
			result, err = s.Login(ctx, "rollback", "correct-password")
			uid, expiry = result.UID, result.SessionExpireAt
			if result.SessionToken != "" {
				t.Error("failed login returned token")
			}
		} else {
			err = s.Logout(ctx, p.SessionToken)
		}
		db.Callback().Update().Remove(name)
		if !errors.Is(err, failure) || uid != "" || expiry != 0 {
			t.Fatal("failed write returned identity", err)
		}
		after := accountRow(t, db, p.UID)
		if after.TokenHash != before.TokenHash || !after.ExpiresAt.Equal(*before.ExpiresAt) || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatal("failed transaction changed previous session")
		}
	}
	if uid, expiry, err := s.Authenticate(ctx, p.SessionToken); err != nil || uid != p.UID || expiry != now.Add(30*24*time.Hour).Unix() {
		t.Fatal("retry after rollback failed", err)
	}
}
