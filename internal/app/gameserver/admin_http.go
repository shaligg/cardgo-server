package gameserver

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/platform/session"
)

// buildAPIMux 组装 gameserver 同进程 HTTP 入口。
//
// 玩家实时玩法不走这里；这里仅承载登录发票、健康检查、指标和受控管理接口。
func buildAPIMux(cfg Config, adminToken string, wsServer *ws.Server, metricsReg *metrics.Registry, sessionManager session.Manager, loginService login.Provider) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		drainMode := wsServer.IsDrainMode()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ready":      !drainMode,
			"drain_mode": drainMode,
			"node_id":    cfg.Server.NodeID,
		})
	})
	mux.Handle("/metricsz", requireAdminToken(cfg.Admin.RequireAuth, adminToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, metricsReg.Snapshot())
	})))
	mux.Handle("/admin/drain", requireAdminToken(cfg.Admin.RequireAuth, adminToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"code": 0,
				"msg":  "ok",
				"data": map[string]interface{}{
					"drain_mode": wsServer.IsDrainMode(),
				},
			})
		case http.MethodPost:
			var req struct {
				Enabled bool `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]interface{}{
					"code": "BAD_REQUEST",
					"msg":  "invalid json",
				})
				return
			}
			wsServer.SetDrainMode(req.Enabled)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"code": 0,
				"msg":  "ok",
				"data": map[string]interface{}{
					"drain_mode": wsServer.IsDrainMode(),
				},
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/admin/sessions", requireAdminToken(cfg.Admin.RequireAuth, adminToken, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		count, err := sessionManager.Count(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"code": "INTERNAL_ERROR",
				"msg":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"active_sessions": count,
				"drain_mode":      wsServer.IsDrainMode(),
			},
		})
	})))
	mux.Handle("/api/login", login.NewHTTPHandler(loginService))
	return mux
}

// requireAdminToken 为同端口上的管理路由增加 Bearer Token 校验。
func requireAdminToken(enabled bool, expectedToken string, next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if expectedToken == "" || !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"code": "UNAUTHORIZED",
				"msg":  "invalid admin token",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
