package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

// WebSearch 处理锁外执行的网页搜索协议。
func (h *BizHandler) WebSearch(ctx context.Context, _ string, payload json.RawMessage) (interface{}, *terrors.BizError) {
	var req protocol.WebSearchRequest
	if len(payload) == 0 || json.Unmarshal(payload, &req) != nil {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "invalid web_search payload"}
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" || len(req.Query) > 200 {
		return nil, &terrors.BizError{Code: terrors.CodeBadRequest, Msg: "query must be 1-200 bytes"}
	}

	result, err := h.Searcher.Search(ctx, req.Query)
	if err != nil {
		return nil, &terrors.BizError{Code: terrors.CodeInternal, Msg: err.Error()}
	}
	return result, nil
}
