package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	"github.com/bigfish/go_orm_1/internal/testutil/accountclient"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type loginResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		WSAddr      string `json:"ws_addr"`
		EnterTicket string `json:"enter_ticket"`
	} `json:"data"`
}

type wsClient struct {
	conn *websocket.Conn
	seq  int
}

func main() {
	runID := uuid.NewString()
	owner := connect("social_owner_" + runID)
	defer owner.conn.Close()
	member := connect("social_member_" + runID)
	defer member.conn.Close()
	outsider := connect("social_outsider_" + runID)
	defer outsider.conn.Close()

	ownerUID := "social_owner_" + runID
	memberUID := "social_member_" + runID

	owner.bizOK(protocol.OpFriendApply, map[string]interface{}{
		"target_uid": memberUID,
		"req_id":     uuid.NewString(),
	})
	member.bizOK(protocol.OpFriendApprove, map[string]interface{}{
		"target_uid": ownerUID,
		"req_id":     uuid.NewString(),
	})
	friends := owner.bizOK(protocol.OpFriendList, map[string]interface{}{})
	fmt.Printf("friend.list => %+v\n", friends)

	created := owner.bizOK(protocol.OpGuildCreate, map[string]interface{}{
		"name":   "猫咪工坊-" + runID[:8],
		"req_id": uuid.NewString(),
	})
	guildID := nestedString(created, "payload", "data", "guild", "guild_id")
	if guildID == "" {
		panic(fmt.Sprintf("guild.create missing guild_id: %+v", created))
	}
	member.bizOK(protocol.OpGuildApplyJoin, map[string]interface{}{
		"guild_id": guildID,
		"req_id":   uuid.NewString(),
	})
	applications := owner.bizOK(protocol.OpGuildListApplications, map[string]interface{}{"guild_id": guildID})
	fmt.Printf("guild.applications => %+v\n", applications)
	owner.bizOK(protocol.OpGuildApproveJoin, map[string]interface{}{
		"guild_id":   guildID,
		"target_uid": memberUID,
		"req_id":     uuid.NewString(),
	})
	guild := member.bizOK(protocol.OpGuildGet, map[string]interface{}{})
	fmt.Printf("guild.get => %+v\n", guild)

	outsider.bizError(protocol.OpChatSend, map[string]interface{}{
		"channel": "guild",
		"content": "should be rejected",
		"req_id":  uuid.NewString(),
	})
	world := owner.bizOK(protocol.OpChatSend, map[string]interface{}{
		"channel": "world",
		"content": "hello world",
		"req_id":  uuid.NewString(),
	})
	guildMessage := member.bizOK(protocol.OpChatSend, map[string]interface{}{
		"channel": "guild",
		"content": "hello guild",
		"req_id":  uuid.NewString(),
	})
	worldHistory := member.bizOK(protocol.OpChatHistory, map[string]interface{}{"channel": "world", "limit": 10})
	guildHistory := owner.bizOK(protocol.OpChatHistory, map[string]interface{}{"channel": "guild", "limit": 10})
	fmt.Printf("chat.world => send=%+v history=%+v\n", world, worldHistory)
	fmt.Printf("chat.guild => send=%+v history=%+v\n", guildMessage, guildHistory)

	owner.bizOK(protocol.OpGuildLeave, map[string]interface{}{"req_id": uuid.NewString()})
	transferred := member.bizOK(protocol.OpGuildGet, map[string]interface{}{})
	if nestedString(transferred, "payload", "data", "guild", "owner_uid") != memberUID {
		panic(fmt.Sprintf("guild leader was not transferred: %+v", transferred))
	}
	fmt.Println("social smoke passed")
}

func connect(account string) *wsClient {
	login, err := login(account)
	if err != nil {
		panic(err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(login.Data.WSAddr, nil)
	if err != nil {
		panic(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	client := &wsClient{conn: conn, seq: 1}
	client.send(map[string]interface{}{
		"seq":  client.seq,
		"type": "auth_req",
		"ts":   time.Now().Unix(),
		"payload": map[string]interface{}{
			"ticket": login.Data.EnterTicket,
		},
	})
	if response := client.receive(); response["type"] != "auth_ack" {
		panic(fmt.Sprintf("auth failed: %+v", response))
	}
	return client
}

func login(account string) (loginResponse, error) {
	raw, err := accountclient.LoginAndEnter("", account)
	var result loginResponse
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	return result, err
}

func (c *wsClient) bizOK(opCode int32, payload map[string]interface{}) map[string]interface{} {
	response := c.biz(opCode, payload)
	if response["type"] != "biz_ack" {
		panic(fmt.Sprintf("op_code %d failed: %+v", opCode, response))
	}
	return response
}

func (c *wsClient) bizError(opCode int32, payload map[string]interface{}) map[string]interface{} {
	response := c.biz(opCode, payload)
	if response["type"] != "error" {
		panic(fmt.Sprintf("op_code %d should fail: %+v", opCode, response))
	}
	return response
}

func (c *wsClient) biz(opCode int32, payload map[string]interface{}) map[string]interface{} {
	// 冒烟按真实客户端节奏发包，避免命中 GameServer 的 5ms 入站限频。
	time.Sleep(10 * time.Millisecond)
	c.seq++
	c.send(map[string]interface{}{
		"seq":     c.seq,
		"type":    "biz_req",
		"op_code": opCode,
		"ts":      time.Now().Unix(),
		"payload": payload,
	})
	return c.receive()
}

func (c *wsClient) send(message interface{}) {
	if err := c.conn.WriteJSON(message); err != nil {
		panic(err)
	}
}

func (c *wsClient) receive() map[string]interface{} {
	var response map[string]interface{}
	if err := c.conn.ReadJSON(&response); err != nil {
		panic(err)
	}
	return response
}

func nestedString(value map[string]interface{}, keys ...string) string {
	var current interface{} = value
	for _, key := range keys {
		object, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current = object[key]
	}
	result, _ := current.(string)
	return result
}
