package handler

import (
	"context"
	"encoding/json"

	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// OpHandler 是单个业务协议的处理函数签名。
//
// 模块 Handler 方法只要满足这个签名，就可以注册到 Router。
type OpHandler func(ctx context.Context, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError)

// ExecutionMode 决定协议是否由 Dispatcher 在外层按玩家分片串行。
type ExecutionMode uint8

const (
	// ExecutionPlayerSerial 是普通玩家协议的默认执行方式。
	ExecutionPlayerSerial ExecutionMode = iota
	// ExecutionHandlerManaged 用于需要在玩家分片锁外等待的协议。
	ExecutionHandlerManaged
)

type route struct {
	handler     OpHandler
	mode        ExecutionMode
	cacheResult bool
}

// Router 是纯业务协议路由表。
//
// 它只维护 op_code 到处理函数的映射，不持有 playerService、assetService 等业务依赖。
type Router struct {
	handlers map[int32]route
}

func NewRouter() *Router {
	return &Router{handlers: make(map[int32]route)}
}

// Register 绑定协议号和具体模块 Handler 方法。
func (r *Router) Register(opCode int32, h OpHandler) {
	r.RegisterWithMode(opCode, h, ExecutionPlayerSerial)
}

// RegisterCached 注册需要按 req_id 缓存近期成功结果的状态变更协议。
func (r *Router) RegisterCached(opCode int32, h OpHandler) {
	r.handlers[opCode] = route{handler: h, mode: ExecutionPlayerSerial, cacheResult: true}
}

// RegisterWithMode 绑定协议号、Handler 和非默认执行方式。
func (r *Router) RegisterWithMode(opCode int32, h OpHandler, mode ExecutionMode) {
	r.handlers[opCode] = route{handler: h, mode: mode}
}

func (r *Router) resolve(opCode int32) (route, bool) {
	route, ok := r.handlers[opCode]
	return route, ok
}

// Handle 根据 op_code 找到具体业务处理函数并执行。
func (r *Router) Handle(ctx context.Context, opCode int32, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	route, ok := r.resolve(opCode)
	if !ok {
		return nil, &terrors.BizError{Code: terrors.CodeUnsupported, Msg: "unsupported op_code"}
	}
	return route.handler(ctx, targetUID, payload)
}
