package repo

import (
	"context"
	"errors"
	"time"

	"github.com/bigfish/go_orm_1/internal/repo/model"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAccountRecordNotFound = errors.New("account record not found")
	ErrAccountIdentityExists = errors.New("account identity exists")
	ErrGuestDeviceExists     = errors.New("guest device exists")
)

type Account struct {
	UID, Status, TokenHash string
	ExpiresAt              *time.Time
	DeviceID               string
	GuestDeviceID          *string
	CreatedAt, UpdatedAt   time.Time
}
type AccountIdentity struct {
	Provider, Subject, UID, PasswordHash string
	CreatedAt                            time.Time
}

// DBAccountRepository 只执行账号持久化，不判断密码、状态或会话有效性。
type DBAccountRepository struct{ db *gorm.DB }

func NewDBAccountRepository(db *gorm.DB) *DBAccountRepository { return &DBAccountRepository{db: db} }

func accountReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAccountRecordNotFound
	}
	return err
}

func (r *DBAccountRepository) Identity(ctx context.Context, provider, subject string) (AccountIdentity, error) {
	var row model.AccountIdentity
	err := r.db.WithContext(ctx).Where("provider = ? AND subject = ?", provider, subject).Take(&row).Error
	return AccountIdentity(row), accountReadError(err)
}

// CreateAccountInTx 与身份写入共用业务事务，失败由调用者回滚。
func (r *DBAccountRepository) CreateAccountInTx(ctx context.Context, tx *gorm.DB, a Account) error {
	row := model.Account(a)
	err := tx.WithContext(ctx).Create(&row).Error
	var duplicate *mysql.MySQLError
	if a.GuestDeviceID != nil && errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrGuestDeviceExists
	}
	return err
}
func (r *DBAccountRepository) CreateIdentityInTx(ctx context.Context, tx *gorm.DB, i AccountIdentity) error {
	row := model.AccountIdentity(i)
	err := tx.WithContext(ctx).Create(&row).Error
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return ErrAccountIdentityExists
	}
	return err
}
func (r *DBAccountRepository) AccountInTx(ctx context.Context, tx *gorm.DB, uid string) (Account, error) {
	var row model.Account
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", uid).Take(&row).Error
	return Account(row), accountReadError(err)
}

// HasIdentityInTx 必须是取得账号锁后的首次一致性读，避免旧快照及身份索引间隙锁。
// 身份写入也须先锁定账号；新建账号和身份必须位于同一事务。
func (r *DBAccountRepository) HasIdentityInTx(ctx context.Context, tx *gorm.DB, uid string) (bool, error) {
	var row model.AccountIdentity
	err := tx.WithContext(ctx).Where("uid = ?", uid).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Guest 只按游客恢复键定位；普通设备记录不能用于认证。
func (r *DBAccountRepository) Guest(ctx context.Context, deviceID string) (Account, error) {
	var row model.Account
	err := r.db.WithContext(ctx).Where("guest_device_id = ?", deviceID).Take(&row).Error
	return Account(row), accountReadError(err)
}

// SaveAccountInTx 更新已锁定账号的凭证及设备字段；NULL/空值也必须写入以支持绑定与退出。
func (r *DBAccountRepository) SaveAccountInTx(ctx context.Context, tx *gorm.DB, a Account) error {
	return tx.WithContext(ctx).Model(&model.Account{}).Where("uid = ?", a.UID).Updates(map[string]interface{}{
		"token_hash": a.TokenHash, "expires_at": a.ExpiresAt,
		"device_id": a.DeviceID, "guest_device_id": a.GuestDeviceID,
	}).Error
}

// CheckSchema 只读探测所需表，避免数据库连通但表未准备时报告启动成功。
func (r *DBAccountRepository) CheckSchema(ctx context.Context) error {
	for _, v := range []interface{}{&model.Account{}, &model.AccountIdentity{}} {
		if err := r.db.WithContext(ctx).Session(&gorm.Session{QueryFields: true}).Limit(0).Find(v).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateAccount 仅由显式开发/测试入口调用，GameServer 建表不包含账号域。
func MigrateAccount(db *gorm.DB) error {
	if db == nil {
		return errors.New("account db is nil")
	}
	return db.AutoMigrate(&model.Account{}, &model.AccountIdentity{})
}
