package ws

import (
	"sync"
	"time"

	dto "github.com/bigfish/go_orm_1/internal/framework/transport/dto"
	"github.com/gorilla/websocket"
)

type Client struct {
	ConnID string
	UID    string
	Conn   *websocket.Conn

	sendQueue chan dto.Envelope
	closed    chan struct{}
	closeOnce sync.Once
	writeWait time.Duration
	codec     EnvelopeCodec
}

func NewClient(connID, uid string, conn *websocket.Conn, sendQueueSize int, writeWait time.Duration, codec EnvelopeCodec) *Client {
	if sendQueueSize <= 0 {
		sendQueueSize = 256
	}
	if writeWait <= 0 {
		writeWait = 10 * time.Second
	}
	if codec == nil {
		codec = JSONEnvelopeCodec{}
	}
	return &Client{
		ConnID:    connID,
		UID:       uid,
		Conn:      conn,
		sendQueue: make(chan dto.Envelope, sendQueueSize),
		closed:    make(chan struct{}),
		writeWait: writeWait,
		codec:     codec,
	}
}

func (c *Client) TryEnqueue(env dto.Envelope) bool {
	select {
	case <-c.closed:
		return false
	default:
	}

	select {
	case c.sendQueue <- env:
		return true
	default:
		return false
	}
}

func (c *Client) RunWritePump() {
	for {
		select {
		case <-c.closed:
			return
		case env := <-c.sendQueue:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(c.writeWait))
			data, err := c.codec.EncodeEnvelope(env)
			if err != nil {
				c.Close()
				return
			}
			if err := c.Conn.WriteMessage(c.codec.MessageType(), data); err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.Conn.Close()
	})
}
