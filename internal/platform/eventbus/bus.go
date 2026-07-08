package eventbus

import (
	"context"
	"sync"
)

type Bus interface {
	Publish(ctx context.Context, evt Event) error
	Subscribe(eventType string, handler Handler)
}

type InProcBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewInProcBus() *InProcBus {
	return &InProcBus{handlers: make(map[string][]Handler)}
}

func (b *InProcBus) Publish(ctx context.Context, evt Event) error {
	b.mu.RLock()
	hs := b.handlers[evt.EventType]
	b.mu.RUnlock()

	for _, h := range hs {
		_ = h(ctx, evt)
	}
	return nil
}

func (b *InProcBus) Subscribe(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}
