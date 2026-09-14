package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/testutil/accountclient"
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
	account := "ws_reconnect_user"
	first := login(account)

	c1, _, err := websocket.DefaultDialer.Dial(first.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	_ = c1.SetReadDeadline(time.Now().Add(5 * time.Second))
	auth1 := auth(c1, first.Data.EnterTicket)
	fmt.Printf("auth#1 => %+v\n", auth1)

	time.Sleep(10 * time.Millisecond)
	reqID := uuid.NewString()
	send(c1, map[string]interface{}{
		"seq":     2,
		"type":    "biz_req",
		"op_code": 1002,
		"ts":      time.Now().Unix(),
		"payload": map[string]interface{}{
			"delta":  88,
			"req_id": reqID,
		},
	})
	fmt.Printf("add_gold => %+v\n", recv(c1))
	_ = c1.Close()

	time.Sleep(400 * time.Millisecond)

	second := login(account)
	c2, _, err := websocket.DefaultDialer.Dial(second.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	defer c2.Close()
	_ = c2.SetReadDeadline(time.Now().Add(5 * time.Second))
	auth2 := auth(c2, second.Data.EnterTicket)
	fmt.Printf("auth#2(reconnect) => %+v\n", auth2)
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
	return recv(conn)
}

func login(account string) loginResp {
	raw, err := accountclient.LoginAndEnter("", account)
	if err != nil {
		panic(err)
	}
	var result loginResp
	if err := json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return result
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
