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
	account := "ws_prototype_" + uuid.NewString()
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

	profile := sendBizOK(conn, 2, 1001, map[string]interface{}{})
	fmt.Printf("player.get_profile => %+v\n", profile)

	levelStart := sendBizOK(conn, 3, 1301, map[string]interface{}{
		"level_id": 1,
		"req_id":   uuid.NewString(),
	})
	fmt.Printf("level.start => %+v\n", levelStart)
	levelSessionID := extractLevelSessionID(levelStart)
	if levelSessionID == "" {
		panic("missing level_session_id in level.start response")
	}

	playCard := sendBizOK(conn, 4, 1302, map[string]interface{}{
		"level_session_id": levelSessionID,
		"card_id":          10001,
		"req_id":           uuid.NewString(),
	})
	fmt.Printf("level.play_card => %+v\n", playCard)

	settle := sendBizOK(conn, 5, 1303, map[string]interface{}{
		"level_session_id": levelSessionID,
		"req_id":           uuid.NewString(),
	})
	fmt.Printf("level.settle => %+v\n", settle)

	// Prototype 配置里第 1 关首通金币不足以同时升级卡牌和工坊，
	// 这里使用调试加金币作为 smoke 测试准备，不代表正式玩法产出。
	topUp := sendBizOK(conn, 6, 1002, map[string]interface{}{
		"delta":  200,
		"req_id": uuid.NewString(),
	})
	fmt.Printf("test_setup.add_gold => %+v\n", topUp)

	cards := sendBizOK(conn, 7, 1201, map[string]interface{}{})
	fmt.Printf("card.get_cards => %+v\n", cards)

	cardUpgrade := sendBizOK(conn, 8, 1203, map[string]interface{}{
		"card_id": 10001,
		"req_id":  uuid.NewString(),
	})
	fmt.Printf("card.upgrade => %+v\n", cardUpgrade)

	workshopOverview := sendBizOK(conn, 9, 1401, map[string]interface{}{})
	fmt.Printf("workshop.get_overview => %+v\n", workshopOverview)

	facilityUpgrade := sendBizOK(conn, 10, 1402, map[string]interface{}{
		"facility_id": "oven",
		"req_id":      uuid.NewString(),
	})
	fmt.Printf("workshop.upgrade_facility => %+v\n", facilityUpgrade)

	claimOffline := sendBizOK(conn, 11, 1403, map[string]interface{}{
		"req_id": uuid.NewString(),
	})
	fmt.Printf("workshop.claim_offline_reward => %+v\n", claimOffline)

	relogin := loginAgain(account)
	fmt.Printf("relogin.auth => %+v\n", relogin)
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

func loginAgain(account string) map[string]interface{} {
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
	return auth(conn, lr.Data.EnterTicket)
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

func extractLevelSessionID(resp map[string]interface{}) string {
	payload, _ := resp["payload"].(map[string]interface{})
	data, _ := payload["data"].(map[string]interface{})
	session, _ := data["session"].(map[string]interface{})
	id, _ := session["level_session_id"].(string)
	return id
}
