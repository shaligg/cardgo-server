package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bigfish/go_orm_1/internal/platform/state"
	"github.com/bigfish/go_orm_1/internal/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SnapshotRepository interface {
	SaveSnapshot(ctx context.Context, snapshot state.Snapshot) error
	LoadSnapshot(ctx context.Context, uid string) (state.Snapshot, bool, error)
}

type DBSnapshotRepository struct {
	db *gorm.DB
}

func NewDBSnapshotRepository(db *gorm.DB) *DBSnapshotRepository {
	return &DBSnapshotRepository{db: db}
}

func (r *DBSnapshotRepository) Migrate() error {
	return r.db.AutoMigrate(&model.PlayerSnapshot{})
}

func (r *DBSnapshotRepository) SaveSnapshot(ctx context.Context, snapshot state.Snapshot) error {
	rec := model.PlayerSnapshot{
		UID:       snapshot.UID,
		Version:   snapshot.Version,
		Payload:   snapshot.Payload,
		UpdatedAt: time.Now(),
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"version", "payload", "updated_at"}),
	}).Create(&rec).Error
}

func (r *DBSnapshotRepository) LoadSnapshot(ctx context.Context, uid string) (state.Snapshot, bool, error) {
	var rec model.PlayerSnapshot
	err := r.db.WithContext(ctx).Where("uid = ?", uid).Take(&rec).Error
	if err == nil {
		return state.Snapshot{
			UID:     rec.UID,
			Version: rec.Version,
			Payload: rec.Payload,
		}, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return state.Snapshot{}, false, nil
	}
	return state.Snapshot{}, false, fmt.Errorf("load snapshot: %w", err)
}
