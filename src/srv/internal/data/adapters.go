package data

import (
	"context"
	"errors"
	"time"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AdapterRepository persists adapter definitions and live instances.
type AdapterRepository interface {
	UpsertAdapter(ctx context.Context, adapter domain.Adapter) error
	GetAdapter(ctx context.Context, id string) (domain.Adapter, bool, error)
	ListAdapters(ctx context.Context) ([]domain.Adapter, error)

	UpsertInstance(ctx context.Context, instance domain.AdapterInstance) error
	GetInstance(ctx context.Context, id string) (domain.AdapterInstance, bool, error)
	ListInstances(ctx context.Context, adapterID string) ([]domain.AdapterInstance, error)
	MarkHeartbeat(ctx context.Context, instanceID string, now time.Time) error
	SetInstanceStatus(ctx context.Context, instanceID, status string) error
}

type gormAdapterRepository struct {
	db *gorm.DB
}

func NewGormAdapterRepository(db *gorm.DB) AdapterRepository {
	return &gormAdapterRepository{db: db}
}

func (r *gormAdapterRepository) UpsertAdapter(ctx context.Context, adapter domain.Adapter) error {
	record := adapterToRecord(adapter)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"type",
			"status",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *gormAdapterRepository) GetAdapter(ctx context.Context, id string) (domain.Adapter, bool, error) {
	var record AdapterRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Adapter{}, false, nil
	}
	if err != nil {
		return domain.Adapter{}, false, err
	}
	return adapterFromRecord(record), true, nil
}

func (r *gormAdapterRepository) ListAdapters(ctx context.Context) ([]domain.Adapter, error) {
	var records []AdapterRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return adaptersFromRecords(records), err
}

func (r *gormAdapterRepository) UpsertInstance(ctx context.Context, instance domain.AdapterInstance) error {
	record := adapterInstanceToRecord(instance)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"adapter_id",
			"provider",
			"callback_addr",
			"capabilities",
			"metadata",
			"status",
			"last_heartbeat_at",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *gormAdapterRepository) GetInstance(ctx context.Context, id string) (domain.AdapterInstance, bool, error) {
	var record AdapterInstanceRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AdapterInstance{}, false, nil
	}
	if err != nil {
		return domain.AdapterInstance{}, false, err
	}
	return adapterInstanceFromRecord(record), true, nil
}

func (r *gormAdapterRepository) ListInstances(ctx context.Context, adapterID string) ([]domain.AdapterInstance, error) {
	var records []AdapterInstanceRecord
	q := r.db.WithContext(ctx).Order("created_at ASC")
	if adapterID != "" {
		q = q.Where("adapter_id = ?", adapterID)
	}
	err := q.Find(&records).Error
	return adapterInstancesFromRecords(records), err
}

func (r *gormAdapterRepository) MarkHeartbeat(ctx context.Context, instanceID string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&AdapterInstanceRecord{}).
		Where("id = ?", instanceID).
		Updates(map[string]interface{}{
			"last_heartbeat_at": &now,
			"status":            domain.StatusActive,
		}).Error
}

func (r *gormAdapterRepository) SetInstanceStatus(ctx context.Context, instanceID, status string) error {
	return r.db.WithContext(ctx).Model(&AdapterInstanceRecord{}).
		Where("id = ?", instanceID).
		Update("status", status).Error
}
