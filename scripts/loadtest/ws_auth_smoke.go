package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type loginResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UID         string `json:"uid"`
		ServerID    string `json:"server_id"`
		WSAddr      string `json:"ws_addr"`
		EnterTicket string `json:"enter_ticket"`
		ExpireAt    int64  `json:"expire_at"`
	} `json:"data"`
}

func main() {
	loginURL := "http://127.0.0.1:8080/api/login"
	if v := os.Getenv("LOGIN_URL"); v != "" {
		loginURL = v
	}

	reqBody := map[string]interface{}{
		"account":    "smoke_user",
		"password":   "demo",
		"client_ip":  "127.0.0.1",
		"client_ver": "1.0.0",
	}
	raw, _ := json.Marshal(reqBody)
	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var lr loginResp
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		panic(err)
	}
	if lr.Code != 0 || lr.Data.EnterTicket == "" || lr.Data.WSAddr == "" {
		panic(fmt.Sprintf("login failed: code=%d msg=%s", lr.Code, lr.Msg))
	}

	conn, _, err := websocket.DefaultDialer.Dial(lr.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	authReq := map[string]interface{}{
		"seq":  1,
		"type": "auth_req",
		"ts":   time.Now().Unix(),
		"payload": map[string]interface{}{
			"ticket": lr.Data.EnterTicket,
		},
	}
	if err := conn.WriteJSON(authReq); err != nil {
		panic(err)
	}

	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		panic(err)
	}
	fmt.Printf("auth response: %+v\n", msg)
}
