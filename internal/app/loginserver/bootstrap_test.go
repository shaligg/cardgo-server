package loginserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/platform/account"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

type stubAccounts struct{ err error }

func (s stubAccounts) Register(context.Context, account.Request) (string, error) { return "u1", s.err }
func (s stubAccounts) Login(context.Context, account.Request) (account.LoginResult, error) {
	return account.LoginResult{UID: "u1", SessionToken: "session", SessionExpireAt: 2000000000}, s.err
}
func (s stubAccounts) Logout(context.Context, string) error { return s.err }
func (s stubAccounts) Authenticate(_ context.Context, token, _ string) (string, int64, error) {
	if token != "session" {
		return "", 0, account.ErrInvalid
	}
	return "u1", 2000000000, s.err
}
func httpTestConfig() HTTPSecurityConfig {
	return HTTPSecurityConfig{RequestsPerMinute: 100, MaxBodyBytes: 4096}
}

func TestBootstrapRejectsMissingConfigAndSecret(t *testing.T) {
	t.Setenv("GAME_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := Bootstrap(context.Background()); err == nil || !strings.Contains(err.Error(), "read loginserver config") {
		t.Fatal(err)
	}
	t.Setenv("GAME_CONFIG", "../../../configs/loginserver.local.yaml")
	t.Setenv("GAME_TICKET_SECRET", "")
	if _, err := Bootstrap(context.Background()); err == nil || !strings.Contains(err.Error(), "secret env GAME_TICKET_SECRET is empty") {
		t.Fatal(err)
	}
	t.Setenv("GAME_TICKET_SECRET", "test-secret")
	t.Setenv("ACCOUNT_DB_DSN", "")
	if _, err := Bootstrap(context.Background()); err == nil || !strings.Contains(err.Error(), "DSN") {
		t.Fatal(err)
	}
}

func TestHTTPRoutesAndTicketContract(t *testing.T) {
	secret := []byte("test-secret")
	registry := &login.StaticNodeRegistry{Nodes: []login.NodeInfo{{ServerID: "node-a", WSAddr: "ws://localhost:8081/ws", Healthy: true, MaxOnline: 2000}}}
	mux := buildHTTPMux(stubAccounts{}, login.Service{Allocator: login.RegistryNodeAllocator{Registry: registry}, Issuer: login.LocalTicketIssuer{TTL: time.Minute, Secret: secret, Issuer: "login-module"}}, httpTestConfig())
	request := func(path, body string, authorization bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		if authorization {
			r.Header.Set("Authorization", "Bearer session")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		path, body string
		want       int
	}{
		{"/api/login", "{", 400}, {"/api/login", "null", 400}, {"/api/enter", "null", 400}, {"/api/login", `{"account":"x","uid":"victim"}`, 400},
		{"/api/login", `{} {}`, 400}, {"/api/login", strings.Repeat(" ", 4097), 413},
		{"/api/reconnect", `{}`, 404}, {"/api/refresh", `{}`, 404}, {"/api/logout", `{}`, 401}, {"/api/logout", `{"refresh_token":"old"}`, 400}, {"/api/enter", `{}`, 401},
	} {
		if w := request(tc.path, tc.body, false); w.Code != tc.want {
			t.Fatalf("%s = %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	w := request("/api/login", `{"account":"user","password":"correct-password"}`, false)
	if w.Code != 200 || strings.Contains(w.Body.String(), "enter_ticket") || !strings.Contains(w.Body.String(), "session_token") || strings.Contains(w.Body.String(), "refresh_token") || strings.Contains(w.Body.String(), "account_token") {
		t.Fatal("login selected node", w.Body.String())
	}
	w = request("/api/enter", `{}`, true)
	var result struct{ Data enterResponse }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.Data.SessionExpireAt != 2000000000 {
		t.Fatal(w.Body.String(), err)
	}
	v := auth.Verifier{Secret: secret, Issuer: "login-module", NonceStore: auth.NewMemoryNonceStore()}
	claims, err := v.Verify(context.Background(), result.Data.EnterTicket, "node-a", time.Now().Unix())
	if err != nil || claims.UID != "u1" {
		t.Fatal("ticket contract", err)
	}
	if w := request("/api/logout", `{}`, true); w.Code != 200 {
		t.Fatal("bearer logout failed", w.Code)
	}
	registry.Nodes = nil
	if w := request("/api/login", `{"account":"user","password":"correct-password"}`, false); w.Code != 200 {
		t.Fatal("login depends on nodes")
	}
	if w := request("/api/enter", `{}`, true); w.Code != 503 || !strings.Contains(w.Body.String(), "NO_AVAILABLE_NODE") {
		t.Fatal(w.Body.String())
	}
}

func TestHTTPErrorRedactionAndRateLimit(t *testing.T) {
	cfg := httpTestConfig()
	cfg.RequestsPerMinute = 1
	mux := buildHTTPMux(stubAccounts{err: errors.New("secret-dsn-and-token")}, nil, cfg)
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"account":"user","password":"correct-password"}`))
		r.Header.Set("X-Forwarded-For", []string{"1.1.1.1", "2.2.2.2"}[i])
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if strings.Contains(w.Body.String(), "secret-dsn") {
			t.Fatal("internal error leaked")
		}
		if i == 0 && w.Code != 503 || i == 1 && w.Code != 429 {
			t.Fatal("spoofed header bypassed rate limit", w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable token response")
		}
	}
}
func TestTrustedProxyChain(t *testing.T) {
	cfg := httpTestConfig()
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	s := newRequestSecurity(cfg)
	for _, tc := range []struct{ peer, header, want string }{
		{"198.51.100.2:42", "1.1.1.1", "198.51.100.2"},
		{"10.0.0.1:42", "1.1.1.1, 198.51.100.2, 10.0.0.2", "198.51.100.2"},
		{"10.0.0.1:42", "invalid", ""},
	} {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.header)
		got, err := s.clientIP(r)
		if got != tc.want || (tc.want == "") != (err != nil) {
			t.Fatal(got, err)
		}
	}
}

// TestDeviceHTTPFlow 使用真实账号服务和 MySQL 验证协议优先级、绑定与进入。
func TestDeviceHTTPFlow(t *testing.T) {
	db := testdb.Open(t, &model.Account{}, &model.AccountIdentity{})
	accounts, err := account.NewService(repo.NewDBAccountRepository(db), idb.NewTxManager(db), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	registry := &login.StaticNodeRegistry{Nodes: []login.NodeInfo{{ServerID: "node-a", WSAddr: "ws://localhost:8081/ws", Healthy: true, MaxOnline: 2000}}}
	mux := buildHTTPMux(accounts, login.Service{Allocator: login.RegistryNodeAllocator{Registry: registry}, Issuer: login.LocalTicketIssuer{TTL: time.Minute, Secret: []byte("test-secret"), Issuer: "login-module"}}, httpTestConfig())
	request := func(path, body string, authorization []string, want int) tokenResponse {
		t.Helper()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		for _, value := range authorization {
			r.Header.Add("Authorization", value)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s: got %d want %d: %s", path, w.Code, want, w.Body.String())
		}
		var response struct{ Data tokenResponse }
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response.Data
	}
	request("/api/login", `{"device_id":"android:http"}`, nil, 401)
	registered := request("/api/register", `{"device_id":"android:http"}`, nil, 200)
	if registered.UID == "" || registered.SessionToken != "" {
		t.Fatal("registration missing UID or issued a session")
	}
	guest := request("/api/login", `{"device_id":"android:http"}`, nil, 200)
	if guest.UID != registered.UID || guest.SessionToken == "" {
		t.Fatal("guest missing identity or token")
	}
	for _, header := range [][]string{{"Bearer invalid"}, {""}, {"Basic abc"}, {"Bearer " + guest.SessionToken, "Bearer invalid"}} {
		request("/api/login", `{"device_id":"android:http"}`, header, 401)
		request("/api/register", `{"account":"http_user","password":"correct-password","device_id":"android:http"}`, header, 401)
	}
	request("/api/register", `{"device_id":"android:http"}`, []string{"Bearer " + guest.SessionToken}, 400)
	request("/api/login", `{"account":"missing","password":"wrong-password","device_id":"android:http"}`, nil, 401)
	request("/api/enter", `{"device_id":"android:http"}`, nil, 401)
	resumed := request("/api/login", `{"account":"missing","password":"wrong-password","device_id":"ios:http"}`, []string{"Bearer " + guest.SessionToken}, 200)
	if resumed.UID != guest.UID || resumed.SessionToken != guest.SessionToken {
		t.Fatal("HTTP session priority changed identity/token")
	}
	bound := request("/api/register", `{"account":"http_user","password":"correct-password","device_id":"android:http"}`, []string{"Bearer " + guest.SessionToken}, 200)
	if bound.UID != guest.UID {
		t.Fatal("binding lost guest UID")
	}
	request("/api/login", `{"device_id":"android:http"}`, nil, 401)
	request("/api/login", `{"device_id":"ios:http"}`, nil, 401)
	fresh := request("/api/register", `{"device_id":"android:http"}`, nil, 200)
	if fresh.UID == "" || fresh.UID == guest.UID || fresh.SessionToken != "" {
		t.Fatal("new guest registration reused bound UID or issued session")
	}
	if again := request("/api/register", `{"device_id":"android:http"}`, nil, 200); again.UID != fresh.UID {
		t.Fatal("repeated registration recreated unbound guest")
	}
	if logged := request("/api/login", `{"device_id":"android:http"}`, nil, 200); logged.UID != fresh.UID {
		t.Fatal("device login did not use new guest")
	}
	request("/api/enter", `{}`, []string{"Bearer " + guest.SessionToken}, 200)
	formal := request("/api/login", `{"account":"http_user","password":"correct-password","device_id":"android:http"}`, nil, 200)
	if formal.UID != guest.UID {
		t.Fatal("password did not recover bound guest")
	}
	entry := request("/api/enter", `{"device_id":"android:enter"}`, []string{"Bearer " + formal.SessionToken}, 200)
	if entry.UID != guest.UID {
		t.Fatal("enter UID changed")
	}
	var row model.Account
	if err := db.Where("uid = ?", guest.UID).Take(&row).Error; err != nil || row.DeviceID != "android:enter" || row.GuestDeviceID != nil {
		t.Fatal("enter did not persist formal device", err)
	}
	if again := request("/api/register", `{"device_id":"android:http"}`, nil, 200); again.UID != fresh.UID {
		t.Fatal("formal login/enter replaced unbound guest")
	}
	if recovered := request("/api/login", `{"device_id":"android:http"}`, nil, 200); recovered.UID != fresh.UID {
		t.Fatal("guest recovery lost after formal login")
	}
	request("/api/logout", `{}`, []string{"Bearer " + formal.SessionToken}, 200)
	request("/api/login", `{"device_id":"android:enter"}`, nil, 401)
}
