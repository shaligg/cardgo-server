package handler

import (
	"context"
	"encoding/json"

	"github.com/bigfish/go_orm_1/internal/framework/dispatcher"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// Dispatcher 是业务协议的第一层入口。
//
// 它负责接收 Envelope 中的 op_code、使用鉴权 uid 按玩家分片串行执行，
// 再把请求交给 Router 做具体协议分发。payload 只保留具体业务参数。
type Dispatcher struct {
	router *Router
	exec   *dispatcher.ShardExecutor
}

func NewDispatcher(router *Router, exec *dispatcher.ShardExecutor) *Dispatcher {
	return &Dispatcher{router: router, exec: exec}
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

	if d.exec == nil {
		return d.router.Handle(ctx, opCode, targetUID, payload)
	}

	var out interface{}
	var bizErr *terrors.BizError
	err := d.exec.Submit(ctx, dispatcher.DomainPlayer, targetUID, func(taskCtx context.Context) error {
		out, bizErr = d.router.Handle(taskCtx, opCode, targetUID, payload)
		return nil
	})
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	return out, bizErr
}
