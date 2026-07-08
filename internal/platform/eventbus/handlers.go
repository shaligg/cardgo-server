package eventbus

import "context"

type Event struct {
	EventID   string
	EventType string
	TraceID   string
	Payload   []byte
}

type Handler func(ctx context.Context, evt Event) error
