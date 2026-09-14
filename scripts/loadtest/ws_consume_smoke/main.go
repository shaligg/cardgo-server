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
	raw, err := accountclient.LoginAndEnter("", account)
	var result loginResp
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
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
