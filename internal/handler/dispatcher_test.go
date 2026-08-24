package handler

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
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

	d := NewDispatcher(router, dispatcher.NewShardExecutor(1))
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
