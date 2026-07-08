package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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
	lr := login("ws_biz_user")

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

	reqID := uuid.NewString()
	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     2,
		"type":    "biz_req",
		"op_code": 1002,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"delta":  100,
			"req_id": reqID,
		},
	})
	fmt.Printf("add_gold #1 => %+v\n", recv(conn))

	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     3,
		"type":    "biz_req",
		"op_code": 1002,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"delta":  100,
			"req_id": reqID,
		},
	})
	fmt.Printf("add_gold #2(idempotent) => %+v\n", recv(conn))

	time.Sleep(10 * time.Millisecond)
	send(conn, map[string]interface{}{
		"seq":     4,
		"type":    "biz_req",
		"op_code": 1001,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{},
	})
	fmt.Printf("get_profile => %+v\n", recv(conn))
}

func login(account string) loginResp {
	body := map[string]interface{}{
		"account":    account,
		"password":   "demo",
		"client_ip":  "127.0.0.1",
		"client_ver": "1.0.0",
	}
	raw, _ := json.Marshal(body)
	resp, err := http.Post("http://127.0.0.1:8080/api/login", "application/json", bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var lr loginResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		panic(err)
	}
	if lr.Code != 0 {
		panic(fmt.Sprintf("login failed: %s", lr.Msg))
	}
	return lr
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
