package account

import (
	"context"
	"errors"
	"strings"
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
			uid, err := s.Register(ctx, Request{Account: " Same_User ", Password: "correct-password"})
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
	uid, err := s.Register(ctx, Request{Account: "player", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, Request{Account: "player", Password: "correct-password"})
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
		if _, _, err := s.Authenticate(ctx, token, ""); err != ErrInvalid {
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
		got, expiry, err := s.Authenticate(ctx, p.SessionToken, "")
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
	p, err = s.Login(ctx, Request{Account: "player", Password: "correct-password"})
	if err != nil || p.UID != uid || p.SessionToken == old.SessionToken {
		t.Fatal("new login identity or token", err)
	}
	if _, _, err := s.Authenticate(ctx, old.SessionToken, ""); err != ErrInvalid {
		t.Fatal("replaced token survived new login", err)
	}
	if err := s.Logout(ctx, old.SessionToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, p.SessionToken, ""); err != nil {
		t.Fatal("old device logout affected current session", err)
	}
	before := accountRow(t, db, p.UID)
	now = *before.ExpiresAt
	if _, _, err := s.Authenticate(ctx, p.SessionToken, ""); err != ErrExpired {
		t.Fatal("exact expiry boundary accepted", err)
	}
	if row := accountRow(t, db, p.UID); !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("expired session revived")
	}
	again, err := s.Login(ctx, Request{Account: "player", Password: "correct-password"})
	if err != nil || again.UID != uid || again.SessionToken == p.SessionToken {
		t.Fatal("reauthentication did not recover same UID", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Logout(ctx, again.SessionToken); err != nil {
			t.Fatal("logout not idempotent", err)
		}
	}
	if _, _, err := s.Authenticate(ctx, again.SessionToken, ""); err != ErrInvalid {
		t.Fatal("revoked session accepted", err)
	}
}

func TestInactiveAccountCannotLoginOrRenew(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	s.now = func() time.Time { return now }
	uid, err := s.Register(ctx, Request{Account: "blocked", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"banned", "deleted"} {
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("status", "active").Error; err != nil {
			t.Fatal(err)
		}
		p, err := s.Login(ctx, Request{Account: "blocked", Password: "correct-password"})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		now = now.Add(24 * time.Hour)
		if _, err := s.Login(ctx, Request{Account: "blocked", Password: "correct-password"}); err != ErrInvalid {
			t.Fatal("inactive login", status, err)
		}
		if _, _, err := s.Authenticate(ctx, p.SessionToken, ""); err != ErrInvalid {
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
	if _, err := s.Register(ctx, Request{Account: "concurrent", Password: "correct-password"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, Request{Account: "concurrent", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan error, 8)
	for i := 0; i < cap(ch); i++ {
		go func() { _, _, err := s.Authenticate(ctx, p.SessionToken, ""); ch <- err }()
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
	if uid, _, err := restarted.Authenticate(ctx, p.SessionToken, ""); err != nil || uid != p.UID {
		t.Fatal("retry after restart failed", err)
	}
	// 两个设备并发密码登录都可返回，只有最后提交的会话能继续使用。
	logins := make(chan LoginResult, 2)
	for i := 0; i < cap(logins); i++ {
		go func() {
			result, err := s.Login(ctx, Request{Account: "concurrent", Password: "correct-password"})
			logins <- result
			ch <- err
		}()
	}
	for i := 0; i < cap(logins); i++ {
		if err := <-ch; err != nil {
			t.Fatal("concurrent login failed", err)
		}
	}
	if _, _, err := restarted.Authenticate(ctx, p.SessionToken, ""); err != ErrInvalid {
		t.Fatal("previous login survived replacement", err)
	}
	valid := 0
	for i := 0; i < cap(logins); i++ {
		candidate := <-logins
		if _, _, err := s.Authenticate(ctx, candidate.SessionToken, ""); err == nil {
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
	go func() { _, _, err := restarted.Authenticate(ctx, p.SessionToken, ""); ch <- err }()
	for i := 0; i < 2; i++ {
		if err := <-ch; err != nil && err != ErrInvalid {
			t.Fatal("concurrent logout/use failed", err)
		}
	}
	if _, _, err := s.Authenticate(ctx, p.SessionToken, ""); err != ErrInvalid {
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
	if _, err := s.Register(ctx, Request{Account: "rollback", Password: "correct-password"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.Login(ctx, Request{Account: "rollback", Password: "correct-password"})
	if err != nil {
		t.Fatal(err)
	}
	before := accountRow(t, db, p.UID)
	now = now.Add(24 * time.Hour)
	if _, err := s.Login(ctx, Request{Account: "rollback", Password: "wrong-password"}); err != ErrInvalid {
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
			uid, expiry, err = s.Authenticate(ctx, p.SessionToken, "")
		} else if operation == "login" {
			var result LoginResult
			result, err = s.Login(ctx, Request{Account: "rollback", Password: "correct-password"})
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
	if uid, expiry, err := s.Authenticate(ctx, p.SessionToken, ""); err != nil || uid != p.UID || expiry != now.Add(30*24*time.Hour).Unix() {
		t.Fatal("retry after rollback failed", err)
	}
}

func TestDeviceGuestBindingAndCredentialPriority(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	if _, err := s.Login(ctx, Request{DeviceID: "android:one"}); err != ErrInvalid {
		t.Fatal("unknown device accepted", err)
	}
	var count int64
	if err := db.Model(&model.Account{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("login created an account", count, err)
	}
	for _, id := range []string{"android:one", "android:two"} {
		uid, err := s.Register(ctx, Request{DeviceID: id})
		if err != nil {
			t.Fatal(err)
		}
		if row := accountRow(t, db, uid); row.TokenHash != "" || row.ExpiresAt != nil {
			t.Fatal("registration issued a session")
		}
	}
	guest, err := s.Login(ctx, Request{DeviceID: "android:one"})
	if err != nil {
		t.Fatal(err)
	}
	// 清除本地会话后可通过原设备恢复；新设备不会找到旧游客。
	again, err := s.Login(ctx, Request{DeviceID: "android:one"})
	if err != nil || again.UID != guest.UID || again.SessionToken == guest.SessionToken {
		t.Fatal("device recovery", again, err)
	}
	guest = again
	other, err := s.Login(ctx, Request{DeviceID: "android:two"})
	if err != nil || other.UID == guest.UID {
		t.Fatal("devices shared guest", err)
	}
	before := accountRow(t, db, guest.UID)
	if uid, err := s.Register(ctx, Request{DeviceID: "android:one"}); err != nil || uid != guest.UID {
		t.Fatal("repeat registration changed UID", err)
	}
	if row := accountRow(t, db, guest.UID); row.TokenHash != before.TokenHash || !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("repeat registration changed session")
	}
	// 无效凭证不能降级，也不能把新的设备关联写入数据库。
	for _, req := range []Request{
		{SessionToken: "bad", DeviceID: "android:new"},
		{Account: "missing", Password: "wrong-password", DeviceID: "android:new"},
		{Account: "missing", DeviceID: "android:new"},
	} {
		if _, err := s.Login(ctx, req); err != ErrInvalid && err != ErrBadRequest {
			t.Fatal("bad credential fell back to device", err)
		}
	}
	if _, err := s.repository.Guest(ctx, "android:new"); err != repo.ErrAccountRecordNotFound {
		t.Fatal("failed authentication recorded device", err)
	}
	// 有效 session 优先于密码，记录最近设备但不改变游客恢复键。
	resumed, err := s.Login(ctx, Request{SessionToken: guest.SessionToken, Account: "missing", Password: "wrong-password", DeviceID: "ios:one"})
	if err != nil || resumed.SessionToken != guest.SessionToken || resumed.UID != guest.UID {
		t.Fatal("session priority or stable renewal", err)
	}
	before = accountRow(t, db, guest.UID)
	if before.DeviceID != "ios:one" || before.GuestDeviceID == nil || *before.GuestDeviceID != "android:one" {
		t.Fatal("session authentication changed guest recovery device")
	}
	if _, err := s.Login(ctx, Request{DeviceID: "ios:one"}); err != ErrInvalid {
		t.Fatal("device record became a guest credential", err)
	}
	uid, err := s.Register(ctx, Request{Account: "player", Password: "correct-password", SessionToken: guest.SessionToken, DeviceID: "android:one"})
	if err != nil || uid != guest.UID {
		t.Fatal("binding changed UID", err)
	}
	if row := accountRow(t, db, uid); row.TokenHash != before.TokenHash || !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("binding changed session")
	}
	if _, _, err := s.Authenticate(ctx, guest.SessionToken, ""); err != nil {
		t.Fatal("binding invalidated guest session", err)
	}
	if row := accountRow(t, db, uid); row.DeviceID != "android:one" || row.GuestDeviceID != nil {
		t.Fatal("binding did not preserve device and release guest key")
	}
	for _, id := range []string{"android:one", "ios:one"} {
		if _, err := s.Login(ctx, Request{DeviceID: id}); err != ErrInvalid {
			t.Fatal("bound account recovered through device", id, err)
		}
	}
	formal, err := s.Login(ctx, Request{Account: "player", Password: "correct-password", DeviceID: "android:two"})
	if err != nil || formal.UID != uid {
		t.Fatal("password did not override device guest", err)
	}
	// 正式登录只记录自己的设备，不覆盖其他游客；切回游客仍恢复原 UID。
	if recovered, err := s.Register(ctx, Request{DeviceID: "android:two"}); err != nil || recovered != other.UID {
		t.Fatal("formal login replaced unbound guest", err)
	}
	if _, err := s.Login(ctx, Request{SessionToken: other.SessionToken, DeviceID: "android:two"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(ctx, other.SessionToken); err != nil {
		t.Fatal(err)
	}
	if recovered, err := s.Login(ctx, Request{DeviceID: "android:two"}); err != nil || recovered.UID != other.UID {
		t.Fatal("device did not follow authenticated guest UID", err)
	}
	// 不带 session 的密码注册明确创建新号，设备号不隐式绑定已有游客。
	registered, err := s.Register(ctx, Request{Account: "another", Password: "correct-password", DeviceID: "android:two"})
	if err != nil || registered == other.UID || registered == guest.UID {
		t.Fatal("password registration used device as identity", err)
	}
	if row := accountRow(t, db, registered); row.DeviceID != "android:two" || row.GuestDeviceID != nil {
		t.Fatal("password registration did not record device or created guest key")
	}
	if device, err := s.repository.Guest(ctx, "android:two"); err != nil || device.UID != other.UID {
		t.Fatal("password registration replaced unbound guest", err)
	}
	db.Model(&model.Account{}).Count(&count)
	if count != 3 {
		t.Fatalf("orphan or replacement account: %d", count)
	}
}

func TestDeviceLoginRejectsFormalIdentityAndInactiveAccount(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	for _, provider := range []string{"local", "apple", "google"} {
		uid, err := s.Register(ctx, Request{DeviceID: provider})
		if err != nil {
			t.Fatal(err)
		}
		// 任意正式身份都阻止设备认证，不依赖平台登录是否已接入。
		if err := db.Create(&model.AccountIdentity{Provider: provider, Subject: "subject", UID: uid}).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login(ctx, Request{DeviceID: provider}); err != ErrInvalid {
			t.Fatal("formal identity bypassed by device", provider, err)
		}
		// 正式绑定必须原子释放恢复键；即使数据异常未释放，上面的登录仍拒绝正式身份。
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("guest_device_id", nil).Error; err != nil {
			t.Fatal(err)
		}
		fresh, err := s.Register(ctx, Request{DeviceID: provider})
		if err != nil || fresh == uid {
			t.Fatal("registration did not create a separate guest", provider, err)
		}
		if again, err := s.Register(ctx, Request{DeviceID: provider}); err != nil || again != fresh {
			t.Fatal("unbound guest recreated", provider, err)
		}
	}
	uid, err := s.Register(ctx, Request{DeviceID: "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"banned", "deleted"} {
		if err := db.Model(&model.Account{}).Where("uid = ?", uid).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := s.Login(ctx, Request{DeviceID: "blocked"}); err != ErrInvalid {
			t.Fatal("inactive guest recovered", err)
		}
		if _, err := s.Register(ctx, Request{DeviceID: "blocked"}); err != ErrInvalid {
			t.Fatal("inactive guest re-registered", err)
		}
	}
	for _, id := range []string{"", "with space", "\n", strings.Repeat("x", 129), "设备"} {
		if _, err := s.Login(ctx, Request{DeviceID: id}); err != ErrBadRequest {
			t.Fatal("invalid device accepted", err)
		}
		if _, err := s.Register(ctx, Request{DeviceID: id}); err != ErrBadRequest {
			t.Fatal("invalid device registered", err)
		}
	}
	// varbinary 保证不因数据库默认大小写不敏感排序规则合并设备。
	first, err := s.Register(ctx, Request{DeviceID: "Case"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Register(ctx, Request{DeviceID: "case"})
	if err != nil || first == second {
		t.Fatal("case-sensitive device identity lost", err)
	}
}

func TestConcurrentDeviceCreationAndBinding(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	type outcome struct {
		uid string
		err error
	}
	registerConcurrent := func() string {
		t.Helper()
		results := make(chan outcome, 8)
		start := make(chan struct{})
		for i := 0; i < cap(results); i++ {
			go func() {
				<-start
				uid, err := s.Register(ctx, Request{DeviceID: "concurrent"})
				results <- outcome{uid, err}
			}()
		}
		close(start)
		uid := ""
		for i := 0; i < cap(results); i++ {
			r := <-results
			if r.err != nil || r.uid == "" || (uid != "" && r.uid != uid) {
				t.Fatal("concurrent device registration", r.err)
			}
			uid = r.uid
		}
		return uid
	}
	uid := registerConcurrent()
	var count int64
	db.Model(&model.Account{}).Count(&count)
	if count != 1 {
		t.Fatalf("concurrent device left %d accounts", count)
	}
	guest, err := s.Login(ctx, Request{DeviceID: "concurrent"})
	if err != nil {
		t.Fatal(err)
	}
	// 两者锁同一账号行：游客先提交则绑定旧 token 失败；绑定先提交则游客失败。
	bound := make(chan error, 1)
	go func() {
		_, err := s.Register(ctx, Request{Account: "bound", Password: "correct-password", SessionToken: guest.SessionToken})
		bound <- err
	}()
	other, deviceErr := s.Login(ctx, Request{DeviceID: "concurrent"})
	bindErr := <-bound
	if bindErr == nil {
		if deviceErr != ErrInvalid {
			t.Fatal("device login survived binding", deviceErr)
		}
	} else {
		if bindErr != ErrInvalid || deviceErr != nil {
			t.Fatal("unexpected concurrency failure", bindErr, deviceErr)
		}
		if _, err := s.Register(ctx, Request{Account: "bound", Password: "correct-password", SessionToken: other.SessionToken}); err != nil {
			t.Fatal(err)
		}
		guest = other
	}
	if _, err := s.Login(ctx, Request{DeviceID: "concurrent"}); err != ErrInvalid {
		t.Fatal("bound account reopened", err)
	}
	before := accountRow(t, db, uid)
	fresh := registerConcurrent()
	if fresh == uid {
		t.Fatal("registration exposed original bound UID")
	}
	if err := db.Model(&model.Account{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("concurrent registration created extra guests", count, err)
	}
	if row := accountRow(t, db, fresh); row.TokenHash != "" || row.ExpiresAt != nil {
		t.Fatal("guest registration issued a session")
	}
	if row := accountRow(t, db, uid); row.TokenHash != before.TokenHash || !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("new guest changed original session")
	}
	if original, _, err := s.Authenticate(ctx, guest.SessionToken, ""); err != nil || original != uid {
		t.Fatal("original session lost", err)
	}
	if original, err := s.verifyPassword(ctx, "bound", "correct-password"); err != nil || original != uid {
		t.Fatal("original identity lost", err)
	}
	p, err := s.Login(ctx, Request{DeviceID: "concurrent"})
	if err != nil || p.UID != fresh {
		t.Fatal("device did not login new guest", err)
	}
	before = accountRow(t, db, fresh)
	if again := registerConcurrent(); again != fresh {
		t.Fatal("unbound guest recreated")
	}
	if row := accountRow(t, db, fresh); row.TokenHash != before.TokenHash || !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("repeat registration changed new guest session")
	}
	// 同一 UID 的两个绑定仍须由账号行锁串行，后者必须读到前者已提交的身份。
	bindings := make(chan error, 2)
	for _, name := range []string{"fresh-one", "fresh-two"} {
		go func(name string) {
			_, err := s.Register(ctx, Request{Account: name, Password: "correct-password", SessionToken: p.SessionToken})
			bindings <- err
		}(name)
	}
	winners := 0
	for i := 0; i < cap(bindings); i++ {
		if err := <-bindings; err == nil {
			winners++
		} else if err != ErrUnavailable {
			t.Fatal("same-account binding failed unexpectedly", err)
		}
	}
	if winners != 1 {
		t.Fatal("same account bound more than once", winners)
	}
}

// TestConcurrentGuestBinding 将不同 UID 的身份检查同步到插入之前，稳定覆盖间隙锁死锁。
func TestConcurrentGuestBinding(t *testing.T) {
	s, db := testService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	requests := make([]Request, 0, 2)
	for _, id := range []string{"bind-one", "bind-two"} {
		if _, err := s.Register(ctx, Request{DeviceID: id}); err != nil {
			t.Fatal(err)
		}
		guest, err := s.Login(ctx, Request{DeviceID: id})
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, Request{Account: id, Password: "correct-password", SessionToken: guest.SessionToken, DeviceID: id})
	}
	arrived, release := make(chan struct{}, len(requests)), make(chan struct{})
	var once sync.Once
	callback := "account_test_binding_barrier"
	if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "AccountIdentity" {
			return
		}
		arrived <- struct{}{}
		if len(arrived) == cap(arrived) {
			once.Do(func() { close(release) })
		}
		select {
		case <-release:
		case <-ctx.Done():
			tx.AddError(ctx.Err())
		}
	}); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, len(requests))
	for _, req := range requests {
		go func(req Request) {
			_, err := s.Register(ctx, req)
			results <- err
		}(req)
	}
	for range requests {
		if err := <-results; err != nil {
			t.Error("independent guest binding failed", err)
		}
	}
	db.Callback().Query().Remove(callback)
	for _, req := range requests {
		uid, _, err := s.Authenticate(ctx, req.SessionToken, "")
		if err != nil {
			t.Fatal("binding changed original session", err)
		}
		row := accountRow(t, db, uid)
		if row.GuestDeviceID != nil || row.DeviceID != req.DeviceID {
			t.Fatal("binding did not release only guest key")
		}
	}
}

func TestDeviceAndBindingFailuresRollBack(t *testing.T) {
	s, db := testService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, Request{DeviceID: "rollback"}); err != nil {
		t.Fatal(err)
	}
	guest, err := s.Login(ctx, Request{DeviceID: "rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, Request{Account: "taken", Password: "correct-password"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, Request{Account: "taken", Password: "correct-password", SessionToken: guest.SessionToken}); err != ErrUnavailable {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, guest.SessionToken, ""); err != nil {
		t.Fatal("conflicting binding revoked guest", err)
	}
	before := accountRow(t, db, guest.UID)
	failure := errors.New("injected account write failure")
	callback := "account_test_device_failure"
	if err := db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "AccountIdentity" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Register(ctx, Request{Account: "fresh", Password: "correct-password", SessionToken: guest.SessionToken})
	db.Callback().Create().Remove(callback)
	if !errors.Is(err, failure) {
		t.Fatal("injection not exercised", err)
	}
	if _, err := s.repository.Identity(ctx, "local", "fresh"); err != repo.ErrAccountRecordNotFound {
		t.Fatal("failed binding left identity", err)
	}
	device, err := s.repository.Guest(ctx, "rollback")
	if err != nil || device.UID != guest.UID || accountRow(t, db, guest.UID).TokenHash != before.TokenHash {
		t.Fatal("failed binding changed guest", err)
	}
	// 绑定写账号字段后失败，身份插入、恢复键释放、设备变更均须回滚。
	if err := db.Callback().Update().After("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Account" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Register(ctx, Request{Account: "fresh", Password: "correct-password", SessionToken: guest.SessionToken, DeviceID: "failed-device"})
	if !errors.Is(err, failure) {
		t.Fatal("binding update injection not exercised", err)
	}
	if _, err := s.repository.Identity(ctx, "local", "fresh"); err != repo.ErrAccountRecordNotFound {
		t.Fatal("failed account update left identity", err)
	}
	_, _, err = s.Authenticate(ctx, guest.SessionToken, "failed-device")
	db.Callback().Update().Remove(callback)
	if !errors.Is(err, failure) {
		t.Fatal("device update injection not exercised", err)
	}
	row := accountRow(t, db, guest.UID)
	if row.DeviceID != before.DeviceID || row.GuestDeviceID == nil || *row.GuestDeviceID != "rollback" || row.TokenHash != before.TokenHash || !row.ExpiresAt.Equal(*before.ExpiresAt) {
		t.Fatal("failed transaction changed account fields")
	}
	if _, err := s.Register(ctx, Request{Account: "fresh", Password: "correct-password", SessionToken: guest.SessionToken}); err != nil {
		t.Fatal(err)
	}
	// 新游客行写入后失败，不得占用唯一恢复键或残留账号。
	if err := db.Callback().Create().After("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Account" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"new-failed-device", "rollback"} {
		if _, err := s.Register(ctx, Request{DeviceID: id}); !errors.Is(err, failure) {
			t.Fatal("guest create injection not exercised", err)
		}
		if _, err := s.repository.Guest(ctx, id); err != repo.ErrAccountRecordNotFound {
			t.Fatal("failed registration left guest", err)
		}
	}
	db.Callback().Create().Remove(callback)
	var count int64
	if err := db.Model(&model.Account{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatal("failed registration left orphan account", count, err)
	}
}
