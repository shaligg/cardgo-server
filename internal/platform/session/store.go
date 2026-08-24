package session

import (
	"context"
)

// Store 是会话持久化接口。
//
// 后续如果需要把在线会话索引放到 Redis，可以实现这个接口替换内存版本。
type Store interface {
	Save(ctx context.Context, s Session) error
	Delete(ctx context.Context, uid string) error
	GetByUID(ctx context.Context, uid string) (Session, bool, error)
}
