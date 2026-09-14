package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LoginResult 只在登录响应中持有明文会话令牌，Repository 仅接收哈希。
type LoginResult struct {
	UID, SessionToken string
	SessionExpireAt   int64
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newToken(uid string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "s." + uid + "." + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func parseToken(token string) (string, error) {
	if len(token) != 82 {
		return "", ErrInvalid
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "s" || len(parts[1]) != 36 || uuid.Validate(parts[1]) != nil {
		return "", ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(raw) != 32 {
		return "", ErrInvalid
	}
	return parts[1], nil
}

// Login 验证身份后原子切换唯一有效会话；写入失败保留旧会话。
func (s *Service) Login(ctx context.Context, name, password string) (LoginResult, error) {
	uid, err := s.verifyPassword(ctx, name, password)
	if err != nil {
		return LoginResult{}, err
	}
	var result LoginResult
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		a, err := s.accountInTx(ctx, tx, uid)
		if err != nil {
			return err
		}
		if a.Status != "active" {
			return ErrInvalid
		}
		token, err := newToken(uid)
		if err != nil {
			return err
		}
		expiry := s.now().UTC().Add(s.sessionIdleTTL)
		a.TokenHash, a.ExpiresAt = tokenHash(token), &expiry
		if err := s.repository.SaveSessionInTx(ctx, tx, a); err != nil {
			return err
		}
		result = LoginResult{UID: uid, SessionToken: token, SessionExpireAt: expiry.Unix()}
		return nil
	})
	if err != nil {
		return LoginResult{}, err
	}
	return result, nil
}

// sessionInTx 锁定账号并核对完整令牌；令牌中的 UID 只负责定位，不能证明身份。
func (s *Service) sessionInTx(ctx context.Context, tx *gorm.DB, uid, token string) (repo.Account, error) {
	a, err := s.accountInTx(ctx, tx, uid)
	if err != nil {
		return a, err
	}
	if subtle.ConstantTimeCompare([]byte(a.TokenHash), []byte(tokenHash(token))) != 1 {
		return a, ErrInvalid
	}
	return a, nil
}

// Authenticate 在同一事务中验证并延长闲置期限；令牌不轮换，丢失响应可安全重试。
func (s *Service) Authenticate(ctx context.Context, token string) (string, int64, error) {
	uid, err := parseToken(token)
	if err != nil {
		return "", 0, err
	}
	var expiresAt int64
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		a, err := s.sessionInTx(ctx, tx, uid, token)
		if err != nil {
			return err
		}
		if a.Status != "active" || a.ExpiresAt == nil {
			return ErrInvalid
		}
		// 等待行锁后再取时钟，避免排队期间过期的凭证被续期。
		now := s.now().UTC()
		if !now.Before(*a.ExpiresAt) {
			return ErrExpired
		}
		if deadline := now.Add(s.sessionIdleTTL); deadline.After(*a.ExpiresAt) {
			a.ExpiresAt = &deadline
		}
		if err := s.repository.SaveSessionInTx(ctx, tx, a); err != nil {
			return err
		}
		expiresAt = a.ExpiresAt.Unix()
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return uid, expiresAt, nil
}

// Logout 幂等清空当前凭证；伪造或已被替换的旧令牌不能清除新令牌。
func (s *Service) Logout(ctx context.Context, token string) error {
	uid, err := parseToken(token)
	if err != nil {
		return nil
	}
	return s.tx.Do(ctx, func(tx *gorm.DB) error {
		a, err := s.sessionInTx(ctx, tx, uid, token)
		if errors.Is(err, ErrInvalid) {
			return nil
		}
		if err != nil {
			return err
		}
		a.TokenHash, a.ExpiresAt = "", nil
		return s.repository.SaveSessionInTx(ctx, tx, a)
	})
}
