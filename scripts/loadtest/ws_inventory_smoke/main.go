package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const basicMaterialItemID int64 = 10001 // mirrors configs/gamedata/items.yaml

type loginResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WSAddr      string `json:"ws_addr"`
		EnterTicket string `json:"enter_ticket"`
	} `json:"data"`
}

func main() {
	lr, err := login("ws_inventory_user")
	if err != nil {
		panic(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(lr.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	send(conn, map[string]interface{}{
		"seq":  1,
		"type": "auth_req",
		"ts":   time.Now().Unix(),
		"payload": map[string]interface{}{
			"ticket": lr.Data.EnterTicket,
		},
	})
	fmt.Printf("auth => %+v\n", recv(conn))

	grantReqID := uuid.NewString()
	sendBiz(conn, 2, 1101, map[string]interface{}{
		"item_id": basicMaterialItemID,
		"count":   5,
		"req_id":  grantReqID,
	})
	fmt.Printf("grant_item #1 => %+v\n", recv(conn))

	sendBiz(conn, 3, 1101, map[string]interface{}{
		"item_id": basicMaterialItemID,
		"count":   5,
		"req_id":  grantReqID,
	})
	fmt.Printf("grant_item #2(idempotent) => %+v\n", recv(conn))

	sendBiz(conn, 4, 1103, map[string]interface{}{
		"item_id": basicMaterialItemID,
		"count":   2,
		"req_id":  uuid.NewString(),
	})
	fmt.Printf("consume_item => %+v\n", recv(conn))

	sendBiz(conn, 5, 1103, map[string]interface{}{
		"item_id": basicMaterialItemID,
		"count":   999999,
		"req_id":  uuid.NewString(),
	})
	fmt.Printf("consume_item(overdraw) => %+v\n", recv(conn))

	sendBiz(conn, 6, 1102, map[string]interface{}{})
	fmt.Printf("get_inventory => %+v\n", recv(conn))
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

func sendBiz(conn *websocket.Conn, seq int, opCode int, payload map[string]interface{}) {
	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     seq,
		"type":    "biz_req",
		"op_code": opCode,
		"ts":      time.Now().Unix(),
		"payload": payload,
	})
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
