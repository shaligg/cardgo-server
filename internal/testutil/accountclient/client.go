// Package accountclient 为本地 smoke 提供正式注册、登录和进入调用，不绕过认证。
package accountclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Session 仅在测试进程内保存凭证，禁止输出到测试日志。
type Session struct {
	UID             string `json:"uid"`
	SessionToken    string `json:"session_token"`
	SessionExpireAt int64  `json:"session_expire_at"`
}

var client = &http.Client{Timeout: 12 * time.Second}
var sessions = struct {
	sync.Mutex
	values map[string]Session
}{values: make(map[string]Session)}

func baseURL(value string) string {
	if value == "" {
		value = os.Getenv("LOGIN_URL")
	}
	if value == "" {
		value = "http://127.0.0.1:8080"
	}
	return strings.TrimSuffix(strings.TrimRight(value, "/"), "/api/login")
}

// Post 返回状态和原始响应，调用者只在确认不含凭证的场景输出正文。
func Post(base, path string, body interface{}, token string) (int, []byte, error) {
	base = baseURL(base)
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return 0, nil, fmt.Errorf("invalid login URL")
	}
	data, err := json.Marshal(body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", base+path, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return response.StatusCode, raw, err
}

// NewSession 为测试名称使用独立、稳定账号名；密码只用于测试环境。
func NewSession(base, name string) (Session, error) {
	sum := sha256.Sum256([]byte(name))
	account := "smoke_" + hex.EncodeToString(sum[:16])
	password := os.Getenv("ACCOUNT_TEST_PASSWORD")
	if password == "" {
		password = "local-smoke-password-v1"
	}
	credentials := map[string]string{"account": account, "password": password}
	status, _, err := Post(base, "/api/register", credentials, "")
	if err != nil {
		return Session{}, err
	}
	if status != 200 && status != 409 {
		return Session{}, fmt.Errorf("register status %d", status)
	}
	status, raw, err := Post(base, "/api/login", credentials, "")
	if err != nil {
		return Session{}, err
	}
	if status != 200 {
		return Session{}, fmt.Errorf("login status %d", status)
	}
	var result struct {
		Code int
		Data Session
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return Session{}, err
	}
	if result.Code != 0 || result.Data.UID == "" || result.Data.SessionToken == "" || result.Data.SessionExpireAt <= 0 {
		return Session{}, fmt.Errorf("invalid account session")
	}
	return result.Data, nil
}

// LoginAndEnter 复用短时 smoke 内的账号登录态，重复调用只重新取得入场票。
func LoginAndEnter(base, name string) ([]byte, error) {
	base = baseURL(base)
	key := base + "\n" + name
	sessions.Lock()
	session, ok := sessions.values[key]
	sessions.Unlock()
	if !ok {
		var err error
		session, err = NewSession(base, name)
		if err != nil {
			return nil, err
		}
		sessions.Lock()
		sessions.values[key] = session
		sessions.Unlock()
	}
	status, raw, err := Post(base, "/api/enter", struct{}{}, session.SessionToken)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("enter status %d", status)
	}
	return raw, nil
}
