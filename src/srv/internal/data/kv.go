package data

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KVStoreRecord is a generic key/value row partitioned by adapter source_id.
// The framework treats Value as opaque bytes; the adapter chooses the encoding.
type KVStoreRecord struct {
	SourceID  string `gorm:"column:source_id;primaryKey;size:128"`
	Key       string `gorm:"column:key;primaryKey;size:255"`
	Value     []byte `gorm:"column:value;type:blob;not null"`
	ExpiresAt int64  `gorm:"column:expires_at;not null;default:0"` // 0 = no TTL
	UpdatedAt int64  `gorm:"column:updated_at;not null;autoUpdateTime:false"`
}

func (KVStoreRecord) TableName() string {
	return "kv_store"
}

// KVStoreRepository abstracts persisted adapter key/value rows.
type KVStoreRepository interface {
	Get(ctx context.Context, sourceID, key string) (KVStoreRecord, bool, error)
	Upsert(ctx context.Context, record KVStoreRecord) error
	Delete(ctx context.Context, sourceID, key string) error
	List(ctx context.Context, sourceID, prefix string) ([]KVStoreRecord, error)
}

type gormKVStoreRepository struct {
	db *gorm.DB
}

func NewGormKVStoreRepository(db *gorm.DB) KVStoreRepository {
	return &gormKVStoreRepository{db: db}
}

func (r *gormKVStoreRepository) Get(ctx context.Context, sourceID, key string) (KVStoreRecord, bool, error) {
	var record KVStoreRecord
	err := r.db.WithContext(ctx).First(&record, "source_id = ? AND key = ?", sourceID, key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return KVStoreRecord{}, false, nil
	}
	if err != nil {
		return KVStoreRecord{}, false, err
	}
	return record, true, nil
}

func (r *gormKVStoreRepository) Upsert(ctx context.Context, record KVStoreRecord) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"value",
			"expires_at",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *gormKVStoreRepository) Delete(ctx context.Context, sourceID, key string) error {
	return r.db.WithContext(ctx).
		Where("source_id = ? AND key = ?", sourceID, key).
		Delete(&KVStoreRecord{}).Error
}

func (r *gormKVStoreRepository) List(ctx context.Context, sourceID, prefix string) ([]KVStoreRecord, error) {
	var records []KVStoreRecord
	query := r.db.WithContext(ctx).Where("source_id = ?", sourceID)
	if prefix != "" {
		query = query.Where("key LIKE ?", prefix+"%")
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
