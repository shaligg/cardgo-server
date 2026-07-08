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

type loginResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WSAddr      string `json:"ws_addr"`
		EnterTicket string `json:"enter_ticket"`
	} `json:"data"`
}

func main() {
	lr, err := login("ws_consume_user")
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

	addReqID := uuid.NewString()
	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     2,
		"type":    "biz_req",
		"op_code": 1002,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"delta":  120,
			"req_id": addReqID,
		},
	})
	fmt.Printf("add_gold => %+v\n", recv(conn))

	consumeReqID := uuid.NewString()
	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     3,
		"type":    "biz_req",
		"op_code": 1003,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"amount": 50,
			"req_id": consumeReqID,
		},
	})
	fmt.Printf("consume_gold => %+v\n", recv(conn))

	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     4,
		"type":    "biz_req",
		"op_code": 1003,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"amount": 999999999,
			"req_id": uuid.NewString(),
		},
	})
	fmt.Printf("consume_gold(overdraw) => %+v\n", recv(conn))

	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     5,
		"type":    "biz_req",
		"op_code": 1001,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{},
	})
	fmt.Printf("get_profile => %+v\n", recv(conn))
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
