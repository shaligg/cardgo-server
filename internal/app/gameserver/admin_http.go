package gameserver

import (
	"encoding/json"
	"net/http"

	"github.com/bigfish/go_orm_1/internal/framework/gateway/ws"
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/bigfish/go_orm_1/internal/platform/session"
)

// buildAPIMux 组装 gameserver 同进程 HTTP 入口。
//
// 玩家实时玩法不走这里；这里仅承载登录发票、健康检查、指标和受控管理接口。
func buildAPIMux(cfg Config, wsServer *ws.Server, metricsReg *metrics.Registry, sessionManager session.Manager, loginService login.Provider) http.Handler {
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
	mux.HandleFunc("/metricsz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, metricsReg.Snapshot())
	})
	mux.HandleFunc("/admin/drain", func(w http.ResponseWriter, r *http.Request) {
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
	})
	mux.HandleFunc("/admin/sessions", func(w http.ResponseWriter, r *http.Request) {
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
	})
	mux.Handle("/api/login", login.NewHTTPHandler(loginService))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
