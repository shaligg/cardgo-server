package loginserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/login"
)

func TestBootstrapRejectsMissingConfigAndSecret(t *testing.T) {
	t.Setenv("GAME_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	if _, err := Bootstrap(context.Background()); err == nil || !strings.Contains(err.Error(), "read loginserver config") {
		t.Fatalf("missing config error = %v", err)
	}
	t.Setenv("GAME_CONFIG", "../../../configs/loginserver.local.yaml")
	t.Setenv("GAME_TICKET_SECRET", "")
	if _, err := Bootstrap(context.Background()); err == nil || !strings.Contains(err.Error(), "secret env GAME_TICKET_SECRET is empty") {
		t.Fatalf("missing secret error = %v", err)
	}
}

func TestHTTPRoutesAndTicketContract(t *testing.T) {
	secret := []byte("test-secret")
	registry := &login.StaticNodeRegistry{Nodes: []login.NodeInfo{{ServerID: "node-a", WSAddr: "ws://localhost:8081/ws", Healthy: true, MaxOnline: 2000}}}
	mux := buildHTTPMux(login.Service{
		Allocator: login.RegistryNodeAllocator{Registry: registry},
		Issuer:    login.LocalTicketIssuer{TTL: time.Minute, Secret: secret, Issuer: "login-module"},
	})
	for _, tt := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/healthz", "", http.StatusOK},
		{"POST", "/healthz", "", http.StatusMethodNotAllowed},
		{"GET", "/api/login", "", http.StatusMethodNotAllowed},
		{"POST", "/api/login", "{", http.StatusBadRequest},
		{"POST", "/api/login", "{}", http.StatusBadRequest},
		{"GET", "/metricsz", "", http.StatusNotFound},
		{"GET", "/admin/sessions", "", http.StatusNotFound},
	} {
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if resp.Code != tt.status {
			t.Fatalf("%s %s = %d, want %d", tt.method, tt.path, resp.Code, tt.status)
		}
	}
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"account":"u1"}`)))
	var result struct {
		Code int
		Data login.LoginResult
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil || resp.Code != http.StatusOK || result.Code != 0 {
		t.Fatalf("login response = %s, err=%v", resp.Body.String(), err)
	}
	verifier := auth.Verifier{Secret: secret, Issuer: "login-module", NonceStore: auth.NewMemoryNonceStore()}
	claims, err := verifier.Verify(context.Background(), result.Data.EnterTicket, result.Data.ServerID, time.Now().Unix())
	if err != nil || claims.UID != "u1" || claims.ExpUnix != result.Data.ExpireAt || result.Data.WSAddr != registry.Nodes[0].WSAddr {
		t.Fatalf("invalid issued ticket: result=%+v err=%v", result, err)
	}
	registry.Nodes = nil
	resp = httptest.NewRecorder()
	mux.ServeHTTP(resp, httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"account":"u1"}`)))
	if resp.Code != http.StatusInternalServerError || !strings.Contains(resp.Body.String(), login.ErrNoAvailableNode.Error()) || strings.Contains(resp.Body.String(), "enter_ticket") {
		t.Fatalf("no-node response = %d %s", resp.Code, resp.Body.String())
	}
}
