package account

import (
	"context"
	"errors"
	"time"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Service 编排账号事务，身份和游戏玩家数据保持独立。
type Service struct {
	repository     *repo.DBAccountRepository
	tx             idb.TransactionRunner
	sessionIdleTTL time.Duration
	dummyHash      string
	now            func() time.Time
}

func NewService(repository *repo.DBAccountRepository, tx idb.TransactionRunner, sessionIdleTTL time.Duration) (*Service, error) {
	if repository == nil || tx == nil || sessionIdleTTL <= 0 {
		return nil, errors.New("invalid account service dependencies or ttl")
	}
	dummy, err := hashPassword("unused-dummy-password")
	if err != nil {
		return nil, err
	}
	return &Service{repository: repository, tx: tx, sessionIdleTTL: sessionIdleTTL, dummyHash: dummy, now: time.Now}, nil
}

// Request 不接受客户端 UID；设备只在没有更强凭证时用于恢复未绑定游客。
type Request struct {
	Account, Password, SessionToken, DeviceID string
}

// Register 统一创建正式/游客账号，或凭游客会话绑定身份并保留 UID 和会话。
func (s *Service) Register(ctx context.Context, req Request) (string, error) {
	if err := validateDeviceID(req.DeviceID); err != nil {
		return "", err
	}
	if req.Account == "" && req.Password == "" && req.SessionToken == "" {
		return s.registerDevice(ctx, req.DeviceID)
	}
	name, err := credentials(req.Account, req.Password)
	if err != nil {
		return "", err
	}
	var uid string
	if req.SessionToken != "" {
		uid, err = parseToken(req.SessionToken)
		if err != nil {
			return "", err
		}
	} else {
		uid = uuid.NewString()
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		return "", err
	}
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		a := repo.Account{UID: uid, Status: "active", DeviceID: req.DeviceID}
		if req.SessionToken != "" {
			var err error
			a, err = s.activeSessionInTx(ctx, tx, uid, req.SessionToken)
			if err != nil {
				return err
			}
			bound, err := s.repository.HasIdentityInTx(ctx, tx, uid)
			if err != nil {
				return err
			}
			if bound {
				return ErrUnavailable
			}
		} else if err := s.repository.CreateAccountInTx(ctx, tx, a); err != nil {
			return err
		}
		if err := s.repository.CreateIdentityInTx(ctx, tx, repo.AccountIdentity{Provider: "local", Subject: name, UID: uid, PasswordHash: hash}); err != nil {
			return err
		}
		if req.SessionToken == "" {
			return nil
		}
		a.GuestDeviceID = nil
		if req.DeviceID != "" {
			a.DeviceID = req.DeviceID
		}
		return s.repository.SaveAccountInTx(ctx, tx, a)
	})
	if errors.Is(err, repo.ErrAccountIdentityExists) {
		return "", ErrUnavailable
	}
	if err != nil {
		return "", err
	}
	return uid, nil
}

// verifyPassword 对未知账号也执行等成本比较，公开错误不暴露账号存在性。
func (s *Service) verifyPassword(ctx context.Context, name, password string) (string, error) {
	name, err := credentials(name, password)
	if err != nil {
		return "", err
	}
	identity, err := s.repository.Identity(ctx, "local", name)
	if err != nil && !errors.Is(err, repo.ErrAccountRecordNotFound) {
		return "", err
	}
	hash := identity.PasswordHash
	if err != nil {
		hash = s.dummyHash
	}
	compareErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil || compareErr != nil {
		return "", ErrInvalid
	}
	return identity.UID, nil
}

// accountInTx 锁定账号，不存在时统一返回认证失败。
func (s *Service) accountInTx(ctx context.Context, tx *gorm.DB, uid string) (repo.Account, error) {
	a, err := s.repository.AccountInTx(ctx, tx, uid)
	if errors.Is(err, repo.ErrAccountRecordNotFound) {
		return repo.Account{}, ErrInvalid
	}
	if err != nil {
		return repo.Account{}, err
	}
	return a, nil
}
