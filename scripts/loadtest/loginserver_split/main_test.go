// 登录拆分的真实进程验收；只有显式提供独立测试库时才执行。
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/app/gameserver"
	"github.com/bigfish/go_orm_1/internal/app/loginserver"
	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/bigfish/go_orm_1/internal/platform/login"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type process struct {
	cmd     *exec.Cmd
	done    chan error
	stopped bool
}

type cluster struct {
	t                             *testing.T
	root, dir, secret, adminToken string
	env                           []string
	redis                         *goredis.Client
	redisProcess                  *process
	gameConfig                    gameserver.Config
	loginConfig                   loginserver.Config
	http                          *http.Client
}

// newCluster 只创建测试自有进程；临时 Redis 不使用开发环境的 6379。
func newCluster(t *testing.T) *cluster {
	t.Helper()
	dsn := os.Getenv("LOGIN_SPLIT_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("未设置 LOGIN_SPLIT_TEST_DB_DSN，跳过真实进程验收")
	}
	// 固定端口用于复用现有 smoke；若被占用直接失败，避免触碰已有服务。
	for _, port := range []string{"8080", "8081", "8082", "8091", "8092"} {
		listener, err := net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			t.Fatalf("test port %s unavailable: %v", port, err)
		}
		_ = listener.Close()
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	c := &cluster{t: t, root: root, dir: t.TempDir(), secret: uuid.NewString(), adminToken: uuid.NewString(), http: &http.Client{Timeout: 12 * time.Second}}
	c.env = append(os.Environ(), "LC_ALL=C", "GAME_DB_DSN="+dsn, "GAME_TICKET_SECRET="+c.secret, "GAME_ADMIN_TOKEN="+c.adminToken)
	for _, name := range []string{"gameserver", "loginserver"} {
		c.run("go", "build", "-o", filepath.Join(c.dir, name), "./cmd/"+name)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redisAddr := listener.Addr().String()
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	c.redisProcess = c.start("redis", "redis-server", "--bind", "127.0.0.1", "--port", strconv.Itoa(port), "--save", "", "--appendonly", "no", "--dir", c.dir)
	c.redis = goredis.NewClient(&goredis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = c.redis.Close() })
	c.wait("Redis ready", 10*time.Second, func() bool { return c.redis.Ping(context.Background()).Err() == nil })
	c.gameConfig, err = gameserver.LoadConfig(filepath.Join(root, "configs/gameserver.local.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c.loginConfig, err = loginserver.LoadConfig(filepath.Join(root, "configs/loginserver.local.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	c.gameConfig.Redis.Addr = redisAddr
	c.gameConfig.Redis.PasswordEnvKey = ""
	c.gameConfig.Admin.RequireAuth = true
	c.loginConfig.Redis.Addr = redisAddr
	c.loginConfig.Redis.PasswordEnvKey = ""
	return c
}

func (c *cluster) run(name string, args ...string) []byte {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = c.root, c.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

func (c *cluster) start(label, binary string, args ...string) *process {
	c.t.Helper()
	file, err := os.CreateTemp(c.dir, label+"-*.log")
	if err != nil {
		c.t.Fatal(err)
	}
	p := &process{cmd: exec.Command(binary, args...), done: make(chan error, 1)}
	p.cmd.Dir, p.cmd.Env = c.root, c.env
	p.cmd.Stdout, p.cmd.Stderr = file, file
	if err := p.cmd.Start(); err != nil {
		_ = file.Close()
		c.t.Fatal(err)
	}
	go func() { p.done <- p.cmd.Wait() }()
	c.t.Cleanup(func() {
		c.stop(p, false)
		_ = file.Close()
		if c.t.Failed() {
			data, _ := os.ReadFile(file.Name())
			c.t.Logf("%s log:\n%s", label, data)
		}
	})
	return p
}

func (c *cluster) stop(p *process, crash bool) {
	c.t.Helper()
	if p.stopped {
		return
	}
	p.stopped = true
	if crash {
		_ = p.cmd.Process.Kill()
	} else {
		_ = p.cmd.Process.Signal(syscall.SIGTERM)
	}
	select {
	case err := <-p.done:
		if err != nil && !crash {
			c.t.Errorf("process stop: %v", err)
		}
	case <-time.After(12 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
		c.t.Error("process did not stop within deadline")
	}
}

func (c *cluster) configFile(name string, cfg interface{}) string {
	c.t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		c.t.Fatal(err)
	}
	path := filepath.Join(c.dir, name+".yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		c.t.Fatal(err)
	}
	return path
}

func (c *cluster) startGame(cfg gameserver.Config) *process {
	c.t.Helper()
	path := c.configFile(cfg.Server.NodeID, cfg)
	previous := c.env
	c.env = append(append([]string{}, c.env...), "GAME_CONFIG="+path)
	p := c.start(cfg.Server.NodeID, filepath.Join(c.dir, "gameserver"))
	c.env = previous
	c.wait(cfg.Server.NodeID+" registered", 15*time.Second, func() bool {
		return c.redis.Exists(context.Background(), cfg.Redis.NodeKeyPrefix+":"+cfg.Server.NodeID).Val() == 1
	})
	return p
}

func (c *cluster) startLogin() *process {
	c.t.Helper()
	path := c.configFile("login", c.loginConfig)
	previous := c.env
	// 登录进程显式不提供 MySQL 或管理 Token，验证其启动边界。
	c.env = append(append([]string{}, c.env...), "GAME_CONFIG="+path, "GAME_DB_DSN=", "GAME_ADMIN_TOKEN=")
	p := c.start("login", filepath.Join(c.dir, "loginserver"))
	c.env = previous
	c.wait("LoginServer ready", 10*time.Second, func() bool {
		resp, err := c.http.Get("http://127.0.0.1:8080/healthz")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
	return p
}

func (c *cluster) wait(label string, timeout time.Duration, check func() bool) {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.t.Fatalf("timeout waiting for %s", label)
}

func (c *cluster) request(method, url, body string, admin bool) (int, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if admin {
		req.Header.Set("Authorization", "Bearer "+c.adminToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return resp.StatusCode, data
}

func (c *cluster) login(uid string) login.LoginResult {
	c.t.Helper()
	body, _ := json.Marshal(map[string]string{"account": uid})
	status, data := c.request("POST", "http://127.0.0.1:8080/api/login", string(body), false)
	var result struct {
		Code int
		Data login.LoginResult
	}
	if err := json.Unmarshal(data, &result); err != nil || status != 200 || result.Code != 0 || result.Data.EnterTicket == "" || result.Data.UID != uid || result.Data.ExpireAt <= time.Now().Unix() {
		c.t.Fatalf("invalid login result: status=%d body=%s err=%v", status, data, err)
	}
	return result.Data
}

type envelope struct {
	Type    string `json:"type"`
	Payload struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	} `json:"payload"`
}

func (c *cluster) auth(wsAddr, ticket, wantCode string) *websocket.Conn {
	c.t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsAddr, nil)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { _ = conn.Close() })
	resp := c.exchange(conn, map[string]interface{}{"seq": 1, "type": "auth_req", "ts": time.Now().Unix(), "payload": map[string]string{"ticket": ticket}})
	if wantCode == "" {
		if resp.Type != "auth_ack" || !resp.Payload.OK {
			c.t.Fatalf("expected auth_ack: %+v", resp)
		}
	} else if resp.Type != "error" || resp.Payload.Code != wantCode {
		c.t.Fatalf("expected %s: %+v", wantCode, resp)
	}
	return conn
}

func (c *cluster) exchange(conn *websocket.Conn, req interface{}) envelope {
	c.t.Helper()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(req); err != nil {
		c.t.Fatal(err)
	}
	var out envelope
	if err := conn.ReadJSON(&out); err != nil {
		c.t.Fatal(err)
	}
	return out
}

func (c *cluster) noAvailableNode() {
	c.t.Helper()
	status, body := c.request("POST", "http://127.0.0.1:8080/api/login", `{"account":"no-node"}`, false)
	if status != 500 || !bytes.Contains(body, []byte("no available game server")) || bytes.Contains(body, []byte("enter_ticket")) {
		c.t.Fatalf("unexpected no-node response: %d %s", status, body)
	}
}

func TestSingleNode(t *testing.T) {
	c := newCluster(t)
	game := c.startGame(c.gameConfig)
	loginProcess := c.startLogin()
	uid := "split-" + uuid.NewString()
	result := c.login(uid)
	if result.ServerID != "node-a" || result.WSAddr != "ws://127.0.0.1:8081/ws" {
		t.Fatalf("unexpected allocation: %+v", result)
	}
	ownerKey := c.gameConfig.Redis.PlayerOwnerKeyPrefix + ":" + uid
	if c.redis.Exists(context.Background(), ownerKey).Val() != 0 {
		t.Fatal("LoginServer wrote player ownership")
	}
	conn := c.auth(result.WSAddr, result.EnterTicket, "")
	if c.redis.HGet(context.Background(), ownerKey, "server_id").Val() != "node-a" {
		t.Fatal("GameServer did not claim ownership")
	}
	if n := c.redis.SCard(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":index").Val(); n != 1 {
		t.Fatalf("registered %d nodes, want 1", n)
	}
	t.Log("single node: login fields, auth_ack, ownership and LoginServer dependency isolation passed")
	for _, path := range []string{"/metricsz", "/admin/drain", "/admin/sessions"} {
		if status, _ := c.request("GET", "http://127.0.0.1:8082"+path, "", false); status != 401 {
			t.Fatalf("unprotected %s", path)
		}
		if status, _ := c.request("GET", "http://127.0.0.1:8082"+path, "", true); status != 200 {
			t.Fatalf("unavailable %s", path)
		}
	}
	if status, _ := c.request("GET", "http://127.0.0.1:8082/healthz", "", false); status != 200 {
		t.Fatal("game health unavailable")
	}
	if status, _ := c.request("POST", "http://127.0.0.1:8082/api/login", `{"account":"u1"}`, true); status != 404 {
		t.Fatal("GameServer exposes login")
	}
	monitorPath := filepath.Join(c.dir, "metrics_dashboard")
	c.run("go", "build", "-o", monitorPath, "./scripts/monitoring/metrics_dashboard")
	monitor := exec.Command(monitorPath, "-once")
	monitor.Dir, monitor.Env = c.root, c.env
	monitorOutput, monitorErr := monitor.CombinedOutput()
	// 退出码 2 表示指标已读取但存在告警；地址不可用是 1，不能混为同一种失败。
	var exitErr *exec.ExitError
	if monitorErr != nil && !(errors.As(monitorErr, &exitErr) && exitErr.ExitCode() == 2) {
		t.Fatalf("default monitoring: %v\n%s", monitorErr, monitorOutput)
	}
	if !bytes.Contains(monitorOutput, []byte("GameServer Metrics")) {
		t.Fatalf("missing metrics: %s", monitorOutput)
	}
	t.Logf("default monitoring:\n%s", monitorOutput)
	for _, script := range []string{"./scripts/loadtest/ws_auth_smoke.go", "./scripts/loadtest/ws_biz_smoke", "./scripts/loadtest/ws_reconnect_smoke", "./scripts/loadtest/ws_prototype_smoke"} {
		c.run("go", "run", script)
		t.Logf("smoke passed: %s", script)
	}
	c.auth(result.WSAddr, result.EnterTicket, "AUTH_REPLAY")
	parts := strings.Split(result.EnterTicket, ".")
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var claims auth.TicketClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	wrong, err := auth.SignTicket(claims, []byte("different-secret"))
	if err != nil {
		t.Fatal(err)
	}
	c.auth(result.WSAddr, wrong, "AUTH_INVALID")
	for key, value := range map[string]interface{}{"uid": "tampered", "server_id": "node-b", "exp": time.Now().Add(time.Hour).Unix(), "nonce": "tampered", "issuer": "tampered"} {
		var modified map[string]interface{}
		if err := json.Unmarshal(raw, &modified); err != nil {
			t.Fatal(err)
		}
		modified[key] = value
		body, err := json.Marshal(modified)
		if err != nil {
			t.Fatal(err)
		}
		c.auth(result.WSAddr, base64.RawURLEncoding.EncodeToString(body)+"."+parts[1], "AUTH_INVALID")
	}
	t.Log("wrong secret, all five claim fields tampering and nonce replay rejected")
	c.stop(loginProcess, false)
	resp := c.exchange(conn, map[string]interface{}{"seq": 2, "type": "biz_req", "op_code": 1001, "ts": time.Now().Unix(), "payload": map[string]interface{}{}})
	if resp.Type != "biz_ack" || !resp.Payload.OK {
		t.Fatalf("online player lost after LoginServer stop: %+v", resp)
	}
	t.Log("online WS business request succeeded after LoginServer stopped")
	c.startLogin()
	c.stop(game, false)
	if c.redis.Exists(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":node-a").Val() != 0 {
		t.Fatal("graceful stop did not remove node")
	}
	c.noAvailableNode()
	c.stop(c.redisProcess, false)
	status, body := c.request("POST", "http://127.0.0.1:8080/api/login", `{"account":"redis-down"}`, false)
	if status != 500 || bytes.Contains(body, []byte("enter_ticket")) {
		t.Fatalf("Redis failure fell back: %d %s", status, body)
	}
	t.Log("graceful removal, no available nodes and Redis outage returned controlled errors")
}

func TestMultiNode(t *testing.T) {
	c := newCluster(t)
	// 缩短测试心跳与 TTL，仍通过真实节点上报观察 drain、满载和异常退出。
	c.gameConfig.Redis.NodeHeartbeatSec = 1
	c.gameConfig.Redis.NodeTTLSec = 3
	a := c.startGame(c.gameConfig)
	c.startLogin()
	uid := "affinity-" + uuid.NewString()
	first := c.login(uid)
	conn := c.auth(first.WSAddr, first.EnterTicket, "")
	_ = conn.Close()
	c.waitOnline("node-a", 0)
	bConfig := c.gameConfig
	bConfig.Server.NodeID = "node-b"
	bConfig.Server.WSPort = 8091
	bConfig.Server.AdminPort = 8092
	bConfig.Server.AdvertisedWSAddr = "ws://127.0.0.1:8091/ws"
	b := c.startGame(bConfig)
	if count := c.redis.SCard(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":index").Val(); count != 2 {
		t.Fatalf("node count=%d", count)
	}
	selected := make(map[string]login.LoginResult)
	for i := 0; i < 40; i++ {
		result := c.login(fmt.Sprintf("distribution-%s-%d", uid, i))
		selected[result.ServerID] = result
	}
	if len(selected) != 2 {
		t.Fatalf("equal-load distribution selected %v", selected)
	}
	for _, result := range selected {
		_ = c.auth(result.WSAddr, result.EnterTicket, "").Close()
	}
	c.waitOnline("node-a", 0)
	c.waitOnline("node-b", 0)
	preferred := c.login(uid)
	if preferred.ServerID != "node-a" {
		t.Fatalf("lost original node preference: %+v", preferred)
	}
	_ = c.auth(bConfig.Server.AdvertisedWSAddr, preferred.EnterTicket, "AUTH_INVALID").Close()
	_ = c.auth(preferred.WSAddr, preferred.EnterTicket, "").Close()
	t.Log("dynamic node discovery, equal-load distribution, both-node auth, original-node affinity and wrong-node ticket rejection passed")
	status, body := c.request("POST", "http://127.0.0.1:8082/admin/drain", `{"enabled":true}`, true)
	if status != 200 {
		t.Fatalf("drain: %d %s", status, body)
	}
	c.wait("drain reported", 5*time.Second, func() bool {
		return c.redis.HGet(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":node-a", "drain").Val() == "1"
	})
	if result := c.login(uid); result.ServerID != "node-b" {
		t.Fatal("draining original node was allocated")
	}
	status, _ = c.request("POST", "http://127.0.0.1:8082/admin/drain", `{"enabled":false}`, true)
	if status != 200 {
		t.Fatal("disable drain failed")
	}
	c.wait("drain cleared", 5*time.Second, func() bool {
		return c.redis.HGet(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":node-a", "drain").Val() == "0"
	})
	c.waitOnline("node-a", 0)
	c.waitOnline("node-b", 0)
	c.checkCapacity(c.gameConfig, "node-b")
	c.waitOnline("node-a", 0)
	c.checkCapacity(bConfig, "node-a")
	c.waitOnline("node-b", 0)
	t.Log("drain fallback and independent 2000-connection limits passed on both GameServers")
	c.stop(a, true)
	c.wait("crashed node TTL expired", 6*time.Second, func() bool {
		return c.redis.Exists(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":node-a").Val() == 0
	})
	if result := c.login(uid); result.ServerID != "node-b" {
		t.Fatal("expired original node was allocated")
	}
	a = c.startGame(c.gameConfig)
	if result := c.login(uid); result.ServerID != "node-a" {
		t.Fatal("LoginServer did not detect recovered original node")
	}
	c.stop(b, false)
	if c.redis.Exists(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":node-b").Val() != 0 {
		t.Fatal("stopped node was not removed")
	}
	for i := 0; i < 10; i++ {
		if result := c.login(fmt.Sprintf("after-stop-%s-%d", uid, i)); result.ServerID != "node-a" {
			t.Fatal("stopped node was allocated")
		}
	}
	c.stop(a, false)
	c.noAvailableNode()
	t.Log("crash TTL exclusion, node recovery without LoginServer restart, graceful removal and all-nodes-offline passed")
}

func (c *cluster) waitOnline(node string, count int) {
	c.t.Helper()
	c.wait(node+" online="+strconv.Itoa(count), 5*time.Second, func() bool {
		return c.redis.HGet(context.Background(), c.gameConfig.Redis.NodeKeyPrefix+":"+node, "online").Val() == strconv.Itoa(count)
	})
}

// checkCapacity 使用真实 WS 占满连接槽；在首帧超时前完成准入与分配断言。
// 这里只验收硬上限，不把短时握手测试当作 2000 在线性能压测。
func (c *cluster) checkCapacity(cfg gameserver.Config, otherNode string) {
	c.t.Helper()
	connections := make([]*websocket.Conn, 2000)
	defer func() {
		for _, conn := range connections {
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()
	jobs := make(chan int)
	failures := make(chan error, len(connections))
	var workers sync.WaitGroup
	for i := 0; i < 32; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second}
			for index := range jobs {
				conn, resp, err := dialer.Dial(cfg.Server.AdvertisedWSAddr, nil)
				if err != nil {
					if resp != nil {
						_ = resp.Body.Close()
					}
					failures <- err
					continue
				}
				connections[index] = conn
			}
		}()
	}
	for index := range connections {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	close(failures)
	for err := range failures {
		c.t.Fatalf("fill connection slots: %v", err)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(cfg.Server.AdvertisedWSAddr, nil)
	if conn != nil {
		_ = conn.Close()
	}
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		c.t.Fatalf("connection 2001 was not rejected: response=%v err=%v", resp, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Contains(body, []byte("SERVER_FULL")) {
		c.t.Fatalf("unexpected full response: %s err=%v", body, err)
	}
	c.waitOnline(cfg.Server.NodeID, 2000)
	if result := c.login("full-" + uuid.NewString()); result.ServerID != otherNode {
		c.t.Fatal("full node was allocated")
	}
	c.t.Logf("%s: 2000 live WS slots, connection 2001 rejected, allocator selected %s", cfg.Server.NodeID, otherNode)
}
