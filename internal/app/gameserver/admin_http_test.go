package gameserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"github.com/bigfish/go_orm_1/internal/platform/session"
)

func TestBuildAPIMuxProtectsManagementRoutes(t *testing.T) {
	cfg := defaultConfig()
	cfg.Admin.RequireAuth = true
	handler := buildAPIMux(
		cfg,
		"admin-secret",
		ws.NewServer(ws.Options{}),
		metrics.NewRegistry(),
		session.NewMemoryManager(),
	)

	for _, path := range []string{"/metricsz", "/admin/drain", "/admin/sessions"} {
		t.Run(path+" rejects missing token", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
			}
		})

		t.Run(path+" accepts valid token", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer admin-secret")
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
			}
		})
	}
}

func TestBuildAPIMuxExposesHealthWithoutLogin(t *testing.T) {
	cfg := defaultConfig()
	cfg.Admin.RequireAuth = true
	handler := buildAPIMux(
		cfg,
		"admin-secret",
		ws.NewServer(ws.Options{}),
		metrics.NewRegistry(),
		session.NewMemoryManager(),
	)

	tests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{method: http.MethodGet, path: "/healthz", status: http.StatusOK},
		{method: http.MethodPost, path: "/api/login", body: `{"account":"u1"}`, status: http.StatusNotFound},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != tt.status {
			t.Fatalf("%s %s status = %d, want %d; body=%s", tt.method, tt.path, resp.Code, tt.status, resp.Body.String())
		}
	}
}

func TestRequireAdminTokenCanBeDisabledForLocalDemo(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/metricsz", nil)
	resp := httptest.NewRecorder()

	requireAdminToken(false, "", next).ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNoContent)
	}
}

func TestRequireAdminTokenRejectsInvalidToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	for _, token := range []string{"wrong-token", ""} {
		req := httptest.NewRequest(http.MethodGet, "/metricsz", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp := httptest.NewRecorder()

		requireAdminToken(true, "admin-secret", next).ServeHTTP(resp, req)

		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("token %q status = %d, want %d", token, resp.Code, http.StatusUnauthorized)
		}
	}
}
