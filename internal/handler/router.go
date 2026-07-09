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

// Router 是纯业务协议路由表。
//
// 它只维护 op_code 到处理函数的映射，不持有 playerService、assetService 等业务依赖。
type Router struct {
	handlers map[int32]OpHandler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[int32]OpHandler)}
}

// Register 绑定协议号和具体模块 Handler 方法。
func (r *Router) Register(opCode int32, h OpHandler) {
	r.handlers[opCode] = h
}

// Handle 根据 op_code 找到具体业务处理函数并执行。
func (r *Router) Handle(ctx context.Context, opCode int32, targetUID string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	h, ok := r.handlers[opCode]
	if !ok {
		return nil, &terrors.BizError{Code: terrors.CodeUnsupported, Msg: "unsupported op_code"}
	}
	return h(ctx, targetUID, payload)
}
