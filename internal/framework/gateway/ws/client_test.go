package ws

import (
	"testing"
	"time"

	dto "github.com/bigfish/go_orm_1/internal/framework/transport/dto"
	"github.com/bigfish/go_orm_1/internal/infra/metrics"
)

func TestEnqueueEnvelopeClosesClientWhenQueueIsFull(t *testing.T) {
	registry := metrics.NewRegistry()
	server := NewServer(Options{Metrics: registry})
	client := NewClient("conn-1", "u1", nil, 1, time.Second, nil)

	if !server.enqueueEnvelope(client, dto.Envelope{Type: dto.TypeBizAck}) {
		t.Fatal("first message should enter the queue")
	}
	if server.enqueueEnvelope(client, dto.Envelope{Type: dto.TypeHeartbeatAck}) {
		t.Fatal("second message should fail when queue is full")
	}

	select {
	case <-client.closed:
	default:
		t.Fatal("slow client was not closed")
	}
	if got := registry.Snapshot().WSQueueKick; got != 1 {
		t.Fatalf("ws_queue_kick = %d, want 1", got)
	}
}
