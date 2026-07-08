package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type loginResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WSAddr      string `json:"ws_addr"`
		EnterTicket string `json:"enter_ticket"`
	} `json:"data"`
}

func main() {
	account := "ws_offline_reward_" + uuid.NewString()
	lr, err := login(account)
	if err != nil {
		panic(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(lr.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))

	authResp := auth(conn, lr.Data.EnterTicket)
	fmt.Printf("auth => %+v\n", authResp)
	uid := extractUID(authResp)
	if uid == "" {
		panic("missing uid in auth response")
	}

	firstOverview := sendBizOK(conn, 2, 1401, map[string]interface{}{})
	fmt.Printf("workshop.get_overview(initial) => %+v\n", firstOverview)

	if err := setOfflineSince(uid, 2*time.Hour); err != nil {
		panic(err)
	}

	overview := sendBizOK(conn, 3, 1401, map[string]interface{}{})
	fmt.Printf("workshop.get_overview(after setup) => %+v\n", overview)
	assertNestedNumber(overview, 7200, "payload", "data", "offline_reward_preview", "offline_seconds")
	assertNestedNumber(overview, 40, "payload", "data", "offline_reward_preview", "gold")
	assertNestedNumber(overview, 2, "payload", "data", "offline_reward_preview", "basic_material")

	claim := sendBizOK(conn, 4, 1403, map[string]interface{}{
		"req_id": uuid.NewString(),
	})
	fmt.Printf("workshop.claim_offline_reward => %+v\n", claim)
	assertNestedNumber(claim, 7200, "payload", "data", "effective_seconds")
	assertNestedNumber(claim, 40, "payload", "data", "gold")
	assertNestedNumber(claim, 2, "payload", "data", "preview", "basic_material")

	inventory := sendBizOK(conn, 5, 1102, map[string]interface{}{})
	fmt.Printf("asset.get_inventory => %+v\n", inventory)
	if !hasInventoryItem(inventory, 10001, 2) {
		panic("missing basic material count 2 in inventory")
	}
}

func login(account string) (loginResp, error) {
	loginURL := os.Getenv("LOGIN_URL")
	if loginURL == "" {
		loginURL = "http://127.0.0.1:8080/api/login"
	}

	body := map[string]interface{}{
		"account":    account,
		"password":   "demo",
		"client_ip":  "127.0.0.1",
		"client_ver": "1.0.0",
	}
	raw, _ := json.Marshal(body)
	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		return loginResp{}, fmt.Errorf("login request failed: %w (hint: start server with `go run ./cmd/gameserver`, or set LOGIN_URL)", err)
	}
	defer resp.Body.Close()

	var lr loginResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return loginResp{}, err
	}
	if lr.Code != 0 {
		return loginResp{}, errors.New("login failed: " + lr.Msg)
	}
	return lr, nil
}

func auth(conn *websocket.Conn, ticket string) map[string]interface{} {
	send(conn, map[string]interface{}{
		"seq":  1,
		"type": "auth_req",
		"ts":   time.Now().Unix(),
		"payload": map[string]interface{}{
			"ticket": ticket,
		},
	})
	resp := recv(conn)
	if resp["type"] != "auth_ack" {
		panic(fmt.Sprintf("auth failed: %+v", resp))
	}
	return resp
}

func sendBizOK(conn *websocket.Conn, seq int, opCode int, payload map[string]interface{}) map[string]interface{} {
	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     seq,
		"type":    "biz_req",
		"op_code": opCode,
		"ts":      time.Now().Unix(),
		"payload": payload,
	})
	resp := recv(conn)
	if resp["type"] != "biz_ack" {
		panic(fmt.Sprintf("op_code %d failed: %+v", opCode, resp))
	}
	return resp
}

func send(conn *websocket.Conn, msg interface{}) {
	if err := conn.WriteJSON(msg); err != nil {
		panic(err)
	}
}

func recv(conn *websocket.Conn) map[string]interface{} {
	var out map[string]interface{}
	if err := conn.ReadJSON(&out); err != nil {
		panic(err)
	}
	return out
}

func extractUID(resp map[string]interface{}) string {
	payload, _ := resp["payload"].(map[string]interface{})
	uid, _ := payload["uid"].(string)
	return uid
}

func setOfflineSince(uid string, duration time.Duration) error {
	dsn := os.Getenv("GAME_DB_DSN")
	if dsn == "" {
		dsn = "file:game_demo.db?cache=shared&_busy_timeout=5000"
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	return db.Model(&model.PlayerWorkshop{}).
		Where("uid = ?", uid).
		Update("last_offline_reward_at", time.Now().Add(-duration)).Error
}

func assertNestedNumber(root map[string]interface{}, want float64, path ...string) {
	var current interface{} = root
	for _, key := range path {
		obj, ok := current.(map[string]interface{})
		if !ok {
			panic(fmt.Sprintf("path %v is not an object at %q: %+v", path, key, current))
		}
		current = obj[key]
	}
	got, ok := current.(float64)
	if !ok || got != want {
		panic(fmt.Sprintf("path %v = %+v, want %.0f", path, current, want))
	}
}

func hasInventoryItem(resp map[string]interface{}, itemID int64, count int64) bool {
	payload, _ := resp["payload"].(map[string]interface{})
	data, _ := payload["data"].(map[string]interface{})
	items, _ := data["items"].([]interface{})
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		if int64(number(item["ItemID"])) == itemID && int64(number(item["Count"])) == count {
			return true
		}
	}
	return false
}

func number(v interface{}) float64 {
	n, _ := v.(float64)
	return n
}
