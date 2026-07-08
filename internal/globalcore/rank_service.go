package globalcore

import "context"

type RankItem struct {
	UID   string
	Score int64
	Rank  int
}

type RankService interface {
	UpdateScore(ctx context.Context, boardID string, uid string, score int64, reqID string) error
	GetTopN(ctx context.Context, boardID string, n int) ([]RankItem, error)
}
