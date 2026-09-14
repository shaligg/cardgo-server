package loginserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/bigfish/go_orm_1/internal/platform/account"
	"github.com/bigfish/go_orm_1/internal/platform/login"
)

// accountProvider 是 HTTP 使用的账号能力，不让 Handler 访问数据库。
type accountProvider interface {
	Register(context.Context, string, string) (string, error)
	Login(context.Context, string, string) (account.LoginResult, error)
	Logout(context.Context, string) error
	Authenticate(context.Context, string) (string, int64, error)
}
type credentialRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}
type tokenResponse struct {
	UID             string `json:"uid"`
	SessionToken    string `json:"session_token"`
	SessionExpireAt int64  `json:"session_expire_at"`
}
type enterResponse struct {
	login.LoginResult
	SessionExpireAt int64 `json:"session_expire_at"`
}
type apiResponse struct {
	Code interface{} `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func tokenDTO(p account.LoginResult) tokenResponse {
	return tokenResponse{p.UID, p.SessionToken, p.SessionExpireAt}
}

type accountHTTP struct {
	account  accountProvider
	entry    login.Provider
	security *requestSecurity
	maxBody  int64
}

func buildHTTPMux(accounts accountProvider, entry login.Provider, cfg HTTPSecurityConfig) http.Handler {
	h := &accountHTTP{account: accounts, entry: entry, security: newRequestSecurity(cfg), maxBody: cfg.MaxBodyBytes}
	mux := http.NewServeMux()
	for _, path := range []string{"/api/register", "/api/login", "/api/logout", "/api/enter"} {
		mux.Handle(path, h)
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"ok\":true}\n")
	})
	return mux
}

func writeAPI(w http.ResponseWriter, status int, code interface{}, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{code, msg, data})
}
func writeAPIError(w http.ResponseWriter, err error) {
	status, code, msg := http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service unavailable"
	var oversized *http.MaxBytesError
	switch {
	case errors.As(err, &oversized):
		status, code, msg = 413, "REQUEST_TOO_LARGE", "request too large"
	case errors.Is(err, account.ErrBadRequest):
		status, code, msg = 400, "BAD_REQUEST", "invalid request"
	case errors.Is(err, account.ErrInvalid):
		status, code, msg = 401, "AUTH_INVALID", "authentication required"
	case errors.Is(err, account.ErrExpired):
		status, code, msg = 401, "AUTH_EXPIRED", "credential expired"
	case errors.Is(err, account.ErrUnavailable):
		status, code, msg = 409, "ACCOUNT_UNAVAILABLE", "account unavailable"
	case errors.Is(err, login.ErrNoAvailableNode):
		status, code, msg = 503, "NO_AVAILABLE_NODE", "no available game server"
	}
	writeAPI(w, status, code, msg, nil)
}
func decodeRequest(w http.ResponseWriter, r *http.Request, max int64, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, max)

	d := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := d.Decode(&raw); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return err
		}
		return account.ErrBadRequest
	}
	if err := d.Decode(new(interface{})); err != io.EOF {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return err
		}
		return account.ErrBadRequest
	}
	if len(raw) == 0 || raw[0] != '{' {
		return account.ErrBadRequest
	}
	fields := json.NewDecoder(bytes.NewReader(raw))
	fields.DisallowUnknownFields()
	if err := fields.Decode(dst); err != nil {
		return account.ErrBadRequest
	}

	return nil
}

// ServeHTTP 只做传输校验、限流和业务调用，公开错误不透传依赖细节。
func (h *accountHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPI(w, 405, "METHOD_NOT_ALLOWED", "method not allowed", nil)
		return
	}
	ip, err := h.security.clientIP(r)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if !h.security.allow(ip) {
		w.Header().Set("Retry-After", "60")
		writeAPI(w, 429, "RATE_LIMITED", "retry later", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	var data interface{}
	switch r.URL.Path {
	case "/api/register", "/api/login":
		var req credentialRequest
		if err = decodeRequest(w, r, h.maxBody, &req); err != nil {
			break
		}
		if r.URL.Path == "/api/register" {
			var uid string
			uid, err = h.account.Register(ctx, req.Account, req.Password)
			data = map[string]string{"uid": uid}
		} else {
			var result account.LoginResult
			result, err = h.account.Login(ctx, req.Account, req.Password)
			data = tokenDTO(result)
		}
	case "/api/enter", "/api/logout":
		if err = decodeRequest(w, r, h.maxBody, &struct{}{}); err != nil {
			break
		}
		auth := strings.Fields(r.Header.Get("Authorization"))
		if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
			err = account.ErrInvalid
			break
		}
		if r.URL.Path == "/api/logout" {
			err = h.account.Logout(ctx, auth[1])
			data = struct{}{}
		} else {
			var uid string
			var expiry int64
			uid, expiry, err = h.account.Authenticate(ctx, auth[1])
			if err == nil {
				var entry login.LoginResult
				entry, err = h.entry.Enter(ctx, uid, ip)
				data = enterResponse{LoginResult: entry, SessionExpireAt: expiry}
			}
		}
	}
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeAPI(w, 200, 0, "ok", data)
}
