package globalcore

import "context"

// NoticeItem 是公告列表返回的轻量 DTO。
type NoticeItem struct {
	NoticeID string
	Content  string
	StartAt  int64
	EndAt    int64
}

// NoticeService 定义公告公共领域能力。
//
// 公告是跨 GameServer 可见的公共数据，发布和查询不能依赖单个物理节点内存。
type NoticeService interface {
	Publish(ctx context.Context, operatorUID string, content string, reqID string) error
	ListActive(ctx context.Context) ([]NoticeItem, error)
}
