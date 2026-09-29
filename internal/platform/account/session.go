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

// Login 只认证已有账号；已提供凭证失败时绝不降级为设备登录。
func (s *Service) Login(ctx context.Context, req Request) (LoginResult, error) {
	if err := validateDeviceID(req.DeviceID); err != nil {
		return LoginResult{}, err
	}
	if req.SessionToken != "" {
		uid, expiry, err := s.Authenticate(ctx, req.SessionToken, req.DeviceID)
		if err != nil {
			return LoginResult{}, err
		}
		return LoginResult{UID: uid, SessionToken: req.SessionToken, SessionExpireAt: expiry}, nil
	}
	guest := req.Account == "" && req.Password == ""
	var uid string
	var err error
	if guest {
		if req.DeviceID == "" {
			return LoginResult{}, ErrBadRequest
		}
		var a repo.Account
		a, err = s.repository.Guest(ctx, req.DeviceID)
		if errors.Is(err, repo.ErrAccountRecordNotFound) {
			return LoginResult{}, ErrInvalid
		}
		uid = a.UID
	} else {
		uid, err = s.verifyPassword(ctx, req.Account, req.Password)
	}
	if err != nil {
		return LoginResult{}, err
	}
	var result LoginResult
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		var a repo.Account
		var err error
		if guest {
			a, err = s.guestAccountInTx(ctx, tx, uid, req.DeviceID)
			if errors.Is(err, errGuestDeviceChanged) {
				return ErrInvalid
			}
		} else {
			a, err = s.accountInTx(ctx, tx, uid)
		}
		if err != nil {
			return err
		}
		if a.Status != "active" {
			return ErrInvalid
		}
		if req.DeviceID != "" {
			a.DeviceID = req.DeviceID
		}
		token, err := newToken(uid)
		if err != nil {
			return err
		}
		expiry := s.now().UTC().Add(s.sessionIdleTTL)
		a.TokenHash, a.ExpiresAt = tokenHash(token), &expiry
		if err := s.repository.SaveAccountInTx(ctx, tx, a); err != nil {
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

// activeSessionInTx 取得账号锁后再检查状态和时限，供绑定与续期共同使用。
func (s *Service) activeSessionInTx(ctx context.Context, tx *gorm.DB, uid, token string) (repo.Account, error) {
	a, err := s.sessionInTx(ctx, tx, uid, token)
	if err != nil {
		return a, err
	}
	if a.Status != "active" || a.ExpiresAt == nil {
		return a, ErrInvalid
	}
	if !s.now().UTC().Before(*a.ExpiresAt) {
		return a, ErrExpired
	}
	return a, nil
}

// Authenticate 在同一事务中验证、记录设备并续期；令牌不轮换。
func (s *Service) Authenticate(ctx context.Context, token, deviceID string) (string, int64, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return "", 0, err
	}
	uid, err := parseToken(token)
	if err != nil {
		return "", 0, err
	}
	var expiresAt int64
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		a, err := s.activeSessionInTx(ctx, tx, uid, token)
		if err != nil {
			return err
		}
		if deviceID != "" {
			a.DeviceID = deviceID
		}
		now := s.now().UTC()
		if deadline := now.Add(s.sessionIdleTTL); deadline.After(*a.ExpiresAt) {
			a.ExpiresAt = &deadline
		}
		if err := s.repository.SaveAccountInTx(ctx, tx, a); err != nil {
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
		return s.repository.SaveAccountInTx(ctx, tx, a)
	})
}
