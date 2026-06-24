package data

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AuthStateRecord preserves the legacy uipath_auth_state table shape.
type AuthStateRecord struct {
	Key       string `gorm:"column:key;primaryKey;size:255"`
	Value     string `gorm:"column:value;type:text;not null"`
	UpdatedAt int64  `gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (AuthStateRecord) TableName() string {
	return "uipath_auth_state"
}

// AuthStateRepository abstracts persisted auth-state key/value rows.
type AuthStateRepository interface {
	Get(ctx context.Context, key string) (AuthStateRecord, bool, error)
	Upsert(ctx context.Context, record AuthStateRecord) error
}

type gormAuthStateRepository struct {
	db *gorm.DB
}

func NewGormAuthStateRepository(db *gorm.DB) AuthStateRepository {
	return &gormAuthStateRepository{db: db}
}

func (r *gormAuthStateRepository) Get(ctx context.Context, key string) (AuthStateRecord, bool, error) {
	var record AuthStateRecord
	err := r.db.WithContext(ctx).First(&record, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AuthStateRecord{}, false, nil
	}
	if err != nil {
		return AuthStateRecord{}, false, err
	}
	return record, true, nil
}

func (r *gormAuthStateRepository) Upsert(ctx context.Context, record AuthStateRecord) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"value",
			"updated_at",
		}),
	}).Create(&record).Error
}
