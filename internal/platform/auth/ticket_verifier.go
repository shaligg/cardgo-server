package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type TicketClaims struct {
	UID      string `json:"uid"`
	ServerID string `json:"server_id"`
	ExpUnix  int64  `json:"exp"`
	Nonce    string `json:"nonce"`
	Issuer   string `json:"issuer"`
}

type TicketVerifier interface {
	Verify(ctx context.Context, token string, expectedServerID string, nowUnix int64) (*TicketClaims, error)
	ConsumeNonceOnce(ctx context.Context, nonce string, ttl time.Duration) error
}

var ErrInvalidToken = errors.New("invalid ticket")
var ErrExpiredToken = errors.New("expired ticket")

type Verifier struct {
	NonceStore NonceStore
	Secret     []byte
	Issuer     string
}

// SignTicket 使用 HMAC-SHA256 签发不可篡改的短期进入票据。
func SignTicket(claims TicketClaims, secret []byte) (string, error) {
	if len(secret) == 0 || claims.UID == "" || claims.ServerID == "" || claims.ExpUnix <= 0 || claims.Nonce == "" || claims.Issuer == "" {
		return "", ErrInvalidToken
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", ErrInvalidToken
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	signature := ticketSignature(payloadPart, secret)
	return payloadPart + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (v Verifier) Verify(ctx context.Context, token string, expectedServerID string, nowUnix int64) (*TicketClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(v.Secret) == 0 || v.Issuer == "" {
		return nil, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !hmac.Equal(signature, ticketSignature(parts[0], v.Secret)) {
		return nil, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims TicketClaims
	if json.Unmarshal(payload, &claims) != nil || claims.UID == "" || claims.ServerID == "" || claims.Nonce == "" || claims.Issuer != v.Issuer {
		return nil, ErrInvalidToken
	}
	if claims.ExpUnix < nowUnix {
		return nil, ErrExpiredToken
	}
	if claims.ServerID != expectedServerID {
		return nil, ErrInvalidToken
	}
	if err := v.ConsumeNonceOnce(ctx, claims.Nonce, time.Until(time.Unix(claims.ExpUnix, 0))); err != nil {
		return nil, err
	}
	return &claims, nil
}

func (v Verifier) ConsumeNonceOnce(ctx context.Context, nonce string, ttl time.Duration) error {
	if v.NonceStore == nil {
		return nil
	}
	return v.NonceStore.ConsumeOnce(ctx, nonce, ttl)
}

func ticketSignature(payload string, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
