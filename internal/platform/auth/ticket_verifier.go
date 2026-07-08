package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

type TicketClaims struct {
	UID      string
	ServerID string
	ExpUnix  int64
	Nonce    string
	Issuer   string
}

type TicketVerifier interface {
	Verify(ctx context.Context, token string, expectedServerID string, nowUnix int64) (*TicketClaims, error)
	ConsumeNonceOnce(ctx context.Context, nonce string, ttl time.Duration) error
}

var ErrInvalidToken = errors.New("invalid ticket")
var ErrExpiredToken = errors.New("expired ticket")

type Verifier struct {
	NonceStore NonceStore
}

func (v Verifier) Verify(ctx context.Context, token string, expectedServerID string, nowUnix int64) (*TicketClaims, error) {
	_ = ctx
	parts := strings.Split(token, ":")
	if len(parts) < 4 || len(parts) > 5 || parts[0] != "demo" {
		return nil, ErrInvalidToken
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if exp < nowUnix {
		return nil, ErrExpiredToken
	}
	if parts[2] != expectedServerID {
		return nil, ErrInvalidToken
	}
	nonce := token
	if len(parts) == 5 && parts[4] != "" {
		nonce = parts[4]
	}
	claims := &TicketClaims{UID: parts[1], ServerID: parts[2], ExpUnix: exp, Nonce: nonce, Issuer: "login-module"}
	if err := v.ConsumeNonceOnce(ctx, claims.Nonce, time.Until(time.Unix(exp, 0))); err != nil {
		return nil, err
	}
	return claims, nil
}

func (v Verifier) ConsumeNonceOnce(ctx context.Context, nonce string, ttl time.Duration) error {
	if v.NonceStore == nil {
		return nil
	}
	return v.NonceStore.ConsumeOnce(ctx, nonce, ttl)
}
