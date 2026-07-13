package globalcore

import "context"

// RankItem 是排行榜查询结果 DTO。
type RankItem struct {
	UID   string
	Score int64
	Rank  int
}

// RankService 定义排行榜公共领域能力。
//
// 请求期提交分数和查榜通过该接口完成；排行奖励分段、奖励生成等可复用规则也应沉到
// globalcore/rank 领域内，避免 LocalService 和 RemoteClient 复制两套业务逻辑。
type RankService interface {
	UpdateScore(ctx context.Context, boardID string, uid string, score int64, reqID string) error
	GetTopN(ctx context.Context, boardID string, n int) ([]RankItem, error)
}
