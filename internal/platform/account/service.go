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

// Register 显式创建账号与本地身份，重复注册回滚新 UID。
func (s *Service) Register(ctx context.Context, name, password string) (string, error) {
	name, err := credentials(name, password)
	if err != nil {
		return "", err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	uid := uuid.NewString()
	err = s.tx.Do(ctx, func(tx *gorm.DB) error {
		if err := s.repository.CreateAccountInTx(ctx, tx, repo.Account{UID: uid, Status: "active"}); err != nil {
			return err
		}
		return s.repository.CreateIdentityInTx(ctx, tx, repo.AccountIdentity{Provider: "local", Subject: name, UID: uid, PasswordHash: hash})
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
