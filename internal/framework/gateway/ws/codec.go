package ws

import (
	"encoding/json"

	dto "github.com/bigfish/go_orm_1/internal/framework/transport/dto"
	"github.com/gorilla/websocket"
)

// EnvelopeCodec 负责 WS 业务包的外层编解码。
//
// 当前实现仍然是 JSON；未来切 protobuf 或自定义二进制时，优先替换这里。
type EnvelopeCodec interface {
	DecodeEnvelope(data []byte) (dto.RawEnvelope, error)
	EncodeEnvelope(env dto.Envelope) ([]byte, error)
	MessageType() int
}

// JSONEnvelopeCodec 是当前 MVP 使用的 JSON 协议实现。
type JSONEnvelopeCodec struct{}

func (JSONEnvelopeCodec) DecodeEnvelope(data []byte) (dto.RawEnvelope, error) {
	var env dto.RawEnvelope
	err := json.Unmarshal(data, &env)
	return env, err
}

func (JSONEnvelopeCodec) EncodeEnvelope(env dto.Envelope) ([]byte, error) {
	return json.Marshal(env)
}

func (JSONEnvelopeCodec) MessageType() int {
	return websocket.TextMessage
}
