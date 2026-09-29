package account

import (
	"context"
	"errors"

	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var errGuestDeviceChanged = errors.New("guest device released")

func validateDeviceID(deviceID string) error {
	if len(deviceID) > 128 {
		return ErrBadRequest
	}
	for _, c := range deviceID {
		if c < '!' || c > '~' {
			return ErrBadRequest
		}
	}
	return nil
}

// registerDevice 复用唯一的未绑定游客；绑定释放恢复键后才可另建游客。
func (s *Service) registerDevice(ctx context.Context, deviceID string) (string, error) {
	if deviceID == "" {
		return "", ErrBadRequest
	}
	for attempt := 0; attempt < 2; attempt++ {
		a, err := s.repository.Guest(ctx, deviceID)
		creating := errors.Is(err, repo.ErrAccountRecordNotFound)
		if err != nil && !creating {
			return "", err
		}
		err = s.tx.Do(ctx, func(tx *gorm.DB) error {
			if !creating {
				_, err := s.guestAccountInTx(ctx, tx, a.UID, deviceID)
				return err
			}
			a = repo.Account{UID: uuid.NewString(), Status: "active", DeviceID: deviceID, GuestDeviceID: &deviceID}
			return s.repository.CreateAccountInTx(ctx, tx, a)
		})
		if errors.Is(err, repo.ErrGuestDeviceExists) || errors.Is(err, errGuestDeviceChanged) {
			continue // 事务已回滚；并发请求只复用胜出的新游客。
		}
		if err != nil {
			return "", err
		}
		return a.UID, nil
	}
	return "", repo.ErrGuestDeviceExists
}

// guestAccountInTx 按 UID 锁账号后复核恢复键及身份，供游客注册与登录共用。
func (s *Service) guestAccountInTx(ctx context.Context, tx *gorm.DB, uid, deviceID string) (repo.Account, error) {
	a, err := s.accountInTx(ctx, tx, uid)
	if err != nil {
		return a, err
	}
	if a.GuestDeviceID == nil || *a.GuestDeviceID != deviceID {
		return a, errGuestDeviceChanged
	}
	if a.Status != "active" {
		return a, ErrInvalid
	}
	bound, err := s.repository.HasIdentityInTx(ctx, tx, a.UID)
	if bound {
		return a, ErrInvalid
	}
	return a, err
}
