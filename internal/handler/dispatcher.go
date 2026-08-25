package handler

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
	"github.com/bigfish/go_orm_1/internal/platform/session"
)

// Dispatcher 是业务协议的第一层入口。
//
// 它负责接收 Envelope 中的 op_code、使用鉴权 uid 按玩家分片串行执行，
// 再把请求交给 Router 做具体协议分发。payload 只保留具体业务参数。
type Dispatcher struct {
	router *Router
	exec   *dispatcher.ShardExecutor
	cache  *session.CommandCache
}

func NewDispatcher(router *Router, exec *dispatcher.ShardExecutor, cache *session.CommandCache) *Dispatcher {
	return &Dispatcher{router: router, exec: exec, cache: cache}
}

// Handle 实现 gateway/ws.BizHandler。
//
// 这里是 WS 收到 type=biz 后的实际业务调用入口；返回时业务已经执行完成。
func (d *Dispatcher) Handle(ctx context.Context, uid string, opCode int32, payload json.RawMessage) (interface{}, *terrors.BizError) {
	if opCode == 0 {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "missing op_code"}
	}

	// 普通玩家 WS 协议只能操作鉴权 uid，不能由 payload.uid 覆盖。
	targetUID := uid
	route, ok := d.router.resolve(opCode)
	if !ok {
		return nil, &terrors.BizError{Code: terrors.CodeUnsupported, Msg: "unsupported op_code"}
	}

	if d.exec == nil || route.mode == ExecutionHandlerManaged {
		return d.execute(ctx, opCode, route, targetUID, payload)
	}

	var out interface{}
	var bizErr *terrors.BizError
	err := d.exec.Submit(ctx, dispatcher.DomainPlayer, targetUID, func(taskCtx context.Context) error {
		out, bizErr = d.execute(taskCtx, opCode, route, targetUID, payload)
		return nil
	})
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	return out, bizErr
}

func (d *Dispatcher) execute(ctx context.Context, opCode int32, route route, uid string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	if !route.cacheResult {
		return route.handler(ctx, uid, payload)
	}
	var request struct {
		ReqID string `json:"req_id"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &request) != nil || request.ReqID == "" {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "missing req_id"}
	}
	payloadHash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if cached, ok := d.cache.Get(uid, request.ReqID); ok {
		if cached.OpCode != opCode || cached.PayloadHash != payloadHash {
			return nil, &terrors.BizError{Code: terrors.CodeRequestIDConflict, Msg: "req_id conflicts with previous request"}
		}
		return cached.Result, nil
	}

	result, bizErr := route.handler(ctx, uid, payload)
	if bizErr != nil {
		return nil, bizErr
	}
	resultJSON, err := json.Marshal(result)
	if err == nil {
		d.cache.Put(uid, request.ReqID, session.CommandResult{
			OpCode:      opCode,
			PayloadHash: payloadHash,
			Result:      resultJSON,
		})
	}
	return result, nil
}
