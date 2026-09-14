package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/testutil/accountclient"
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
	raw, err := accountclient.LoginAndEnter("", account)
	var result loginResp
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
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
