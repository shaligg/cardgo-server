package handler

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	"github.com/bigfish/go_orm_1/internal/platform/session"
)

func TestHandlerManagedRouteRunsOutsidePlayerShard(t *testing.T) {
	router := NewRouter()
	locked := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	router.Register(1, func(context.Context, string, json.RawMessage) (interface{}, *terrors.BizError) {
		close(locked)
		<-release
		return nil, nil
	})
	router.RegisterWithMode(2, func(context.Context, string, json.RawMessage) (interface{}, *terrors.BizError) {
		return "search-result", nil
	}, ExecutionHandlerManaged)

	d := NewDispatcher(router, dispatcher.NewShardExecutor(1), nil)
	normalDone := make(chan struct{})
	go func() {
		defer close(normalDone)
		d.Handle(context.Background(), "u1", 1, nil)
	}()
	<-locked

	managedDone := make(chan interface{}, 1)
	go func() {
		result, _ := d.Handle(context.Background(), "u1", 2, nil)
		managedDone <- result
	}()

	select {
	case result := <-managedDone:
		if result != "search-result" {
			t.Fatalf("result = %#v", result)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("handler-managed route waited for player shard")
	}

	releaseOnce.Do(func() { close(release) })
	<-normalDone
}

func TestCachedRouteReturnsFirstResultWithoutCallingHandlerAgain(t *testing.T) {
	router := NewRouter()
	calls := 0
	router.RegisterCached(1, func(context.Context, string, json.RawMessage) (interface{}, *terrors.BizError) {
		calls++
		return map[string]int{"value": calls}, nil
	})
	d := NewDispatcher(router, dispatcher.NewShardExecutor(1), session.NewCommandCache(time.Minute, 10, 1024))
	payload := json.RawMessage(`{"req_id":"r1","amount":1}`)

	first, firstErr := d.Handle(context.Background(), "u1", 1, payload)
	second, secondErr := d.Handle(context.Background(), "u1", 1, payload)
	if firstErr != nil || secondErr != nil {
		t.Fatalf("errors = %#v, %#v", firstErr, secondErr)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want 1", calls)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("results differ: %s != %s", firstJSON, secondJSON)
	}
}

func TestCachedRouteRejectsReusedRequestIDWithDifferentPayload(t *testing.T) {
	router := NewRouter()
	router.RegisterCached(1, func(context.Context, string, json.RawMessage) (interface{}, *terrors.BizError) {
		return "ok", nil
	})
	d := NewDispatcher(router, nil, session.NewCommandCache(time.Minute, 10, 1024))
	if _, err := d.Handle(context.Background(), "u1", 1, json.RawMessage(`{"req_id":"r1","amount":1}`)); err != nil {
		t.Fatalf("first request returned error: %#v", err)
	}
	_, bizErr := d.Handle(context.Background(), "u1", 1, json.RawMessage(`{"req_id":"r1","amount":2}`))
	if bizErr == nil || bizErr.Code != terrors.CodeRequestIDConflict {
		t.Fatalf("error = %#v, want request id conflict", bizErr)
	}
}

func TestCachedRouteRequiresRequestID(t *testing.T) {
	router := NewRouter()
	router.RegisterCached(1, func(context.Context, string, json.RawMessage) (interface{}, *terrors.BizError) {
		return "ok", nil
	})
	d := NewDispatcher(router, nil, session.NewCommandCache(time.Minute, 10, 1024))
	_, bizErr := d.Handle(context.Background(), "u1", 1, json.RawMessage(`{"amount":1}`))
	if bizErr == nil || bizErr.Code != terrors.CodeBadRequest {
		t.Fatalf("error = %#v, want bad request", bizErr)
	}
}
