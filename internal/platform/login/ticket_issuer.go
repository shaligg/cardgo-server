package login

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TicketIssuer interface {
	Issue(ctx context.Context, uid string, serverID string) (token string, expireAt int64, err error)
}

type LocalTicketIssuer struct {
	TTL time.Duration
}

func (i LocalTicketIssuer) Issue(ctx context.Context, uid string, serverID string) (string, int64, error) {
	_ = ctx
	exp := time.Now().Add(i.TTL).Unix()
	nonce := uuid.NewString()
	token := fmt.Sprintf("demo:%s:%s:%d:%s", uid, serverID, exp, nonce)
	return token, exp, nil
}
