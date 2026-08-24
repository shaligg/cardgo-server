package login

import (
	"context"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/auth"
	"github.com/google/uuid"
)

type TicketIssuer interface {
	Issue(ctx context.Context, uid string, serverID string) (token string, expireAt int64, err error)
}

type LocalTicketIssuer struct {
	TTL    time.Duration
	Secret []byte
	Issuer string
}

func (i LocalTicketIssuer) Issue(ctx context.Context, uid string, serverID string) (string, int64, error) {
	_ = ctx
	exp := time.Now().Add(i.TTL).Unix()
	token, err := auth.SignTicket(auth.TicketClaims{
		UID:      uid,
		ServerID: serverID,
		ExpUnix:  exp,
		Nonce:    uuid.NewString(),
		Issuer:   i.Issuer,
	}, i.Secret)
	if err != nil {
		return "", 0, err
	}
	return token, exp, nil
}
