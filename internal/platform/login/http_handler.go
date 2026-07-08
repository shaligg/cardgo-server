package login

import (
	"encoding/json"
	"net/http"
)

type loginAPIRequest struct {
	Account   string `json:"account"`
	Password  string `json:"password"`
	ClientIP  string `json:"client_ip"`
	ClientVer string `json:"client_ver"`
}

type loginAPIResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func NewHTTPHandler(provider Provider) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, loginAPIResponse{Code: 1, Msg: "method not allowed"})
			return
		}

		var req loginAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, loginAPIResponse{Code: 1, Msg: "invalid json"})
			return
		}
		if req.Account == "" {
			writeJSON(w, http.StatusBadRequest, loginAPIResponse{Code: 1, Msg: "account is required"})
			return
		}
		if req.ClientIP == "" {
			req.ClientIP = r.Header.Get("X-Forwarded-For")
		}
		result, err := provider.LoginAndIssueTicket(r.Context(), LoginRequest{
			Account:   req.Account,
			Password:  req.Password,
			ClientIP:  req.ClientIP,
			ClientVer: req.ClientVer,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, loginAPIResponse{Code: 1, Msg: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, loginAPIResponse{Code: 0, Msg: "ok", Data: result})
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
