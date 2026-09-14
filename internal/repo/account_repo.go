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
)

type Account struct {
	UID, Status, TokenHash string
	ExpiresAt              *time.Time
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
	return tx.WithContext(ctx).Create(&row).Error
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

// SaveSessionInTx 只更新已锁定账号的当前凭证，空值也必须写入以支持退出。
func (r *DBAccountRepository) SaveSessionInTx(ctx context.Context, tx *gorm.DB, a Account) error {
	return tx.WithContext(ctx).Model(&model.Account{}).Where("uid = ?", a.UID).Updates(map[string]interface{}{
		"token_hash": a.TokenHash, "expires_at": a.ExpiresAt,
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
