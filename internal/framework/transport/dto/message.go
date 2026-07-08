package dto

import "encoding/json"

type Envelope struct {
	Seq     int64       `json:"seq"`
	Type    string      `json:"type"`
	OpCode  int32       `json:"op_code,omitempty"`
	TS      int64       `json:"ts"`
	TraceID string      `json:"trace_id,omitempty"`
	Payload interface{} `json:"payload"`
}

type RawEnvelope struct {
	Seq     int64           `json:"seq"`
	Type    string          `json:"type"`
	OpCode  int32           `json:"op_code,omitempty"`
	TS      int64           `json:"ts"`
	TraceID string          `json:"trace_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

type AuthReqPayload struct {
	Ticket string `json:"ticket"`
}

// AuthAckPayload 是 GameServer 首帧鉴权成功后的返回。
//
// MVP 阶段 session_id 表示本次连接会话 ID，服务端内部当前与连接 ID 使用同一个 UUID。
type AuthAckPayload struct {
	OK        bool                   `json:"ok"`
	UID       string                 `json:"uid,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Resync    map[string]interface{} `json:"resync,omitempty"`
}

type ErrorPayload struct {
	Code string `json:"code"`
	Msg  string `json:"msg,omitempty"`
}

type ServerFullPayload struct {
	Code          string        `json:"code"`
	RetryAfterSec int           `json:"retry_after_sec"`
	Candidates    []interface{} `json:"candidates"`
}

const (
	TypeAuthReq      = "auth_req"
	TypeAuthAck      = "auth_ack"
	TypeHeartbeatReq = "heartbeat_req"
	TypeHeartbeatAck = "heartbeat_ack"
	TypeBizReq       = "biz_req"
	TypeBizAck       = "biz_ack"
	TypePush         = "push"
	TypeError        = "error"
	TypeKick         = "kick"
	TypeServerFull   = "server_full"
)
