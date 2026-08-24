package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
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
	lr, err := login("ws_level_user")
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

	startReqID := uuid.NewString()
	startPayload := map[string]interface{}{
		"level_id": 1,
		"req_id":   startReqID,
	}
	sendBiz(conn, 2, 1301, startPayload)
	startResp := recv(conn)
	fmt.Printf("level.start => %+v\n", startResp)
	levelSessionID := extractLevelSessionID(startResp)
	if levelSessionID == "" {
		panic("missing level_session_id in level.start response")
	}

	sendBiz(conn, 3, 1301, startPayload)
	startRetryResp := recv(conn)
	fmt.Printf("level.start(retry) => %+v\n", startRetryResp)
	if retriedID := extractLevelSessionID(startRetryResp); retriedID != levelSessionID {
		panic(fmt.Sprintf("level.start retry created another session: first=%s retry=%s", levelSessionID, retriedID))
	}

	playReqID := uuid.NewString()
	playPayload := map[string]interface{}{
		"level_session_id": levelSessionID,
		"card_id":          10001,
		"req_id":           playReqID,
	}
	sendBiz(conn, 4, 1302, playPayload)
	playResp := recv(conn)
	fmt.Printf("level.play_card => %+v\n", playResp)

	sendBiz(conn, 5, 1302, playPayload)
	playRetryResp := recv(conn)
	fmt.Printf("level.play_card(retry) => %+v\n", playRetryResp)
	if !reflect.DeepEqual(extractBizData(playResp), extractBizData(playRetryResp)) {
		panic("level.play_card retry returned a different result")
	}

	sendBiz(conn, 6, 1303, map[string]interface{}{
		"level_session_id": levelSessionID,
		"req_id":           uuid.NewString(),
	})
	fmt.Printf("level.settle => %+v\n", recv(conn))
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

func extractLevelSessionID(resp map[string]interface{}) string {
	data := extractBizData(resp)
	session, _ := data["session"].(map[string]interface{})
	id, _ := session["level_session_id"].(string)
	return id
}

func extractBizData(resp map[string]interface{}) map[string]interface{} {
	payload, _ := resp["payload"].(map[string]interface{})
	data, _ := payload["data"].(map[string]interface{})
	return data
}
