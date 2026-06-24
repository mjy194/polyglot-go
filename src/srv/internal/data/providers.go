package data

import (
	"context"
	"errors"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProviderRepository persists direct upstream provider configuration.
type ProviderRepository interface {
	UpsertProvider(ctx context.Context, provider domain.Provider) (domain.Provider, error)
	GetProvider(ctx context.Context, id string) (domain.Provider, bool, error)
	ListProviders(ctx context.Context) ([]domain.Provider, error)
	UpsertCredential(ctx context.Context, credential domain.ProviderCredential) error
	UpsertModelMapping(ctx context.Context, mapping domain.ModelMapping) (domain.ModelMapping, error)
	ListModelMappings(ctx context.Context) ([]domain.ModelMapping, error)
	ListModelMappingsByProvider(ctx context.Context, providerID string) ([]domain.ModelMapping, error)
	DeleteModelMapping(ctx context.Context, id string) error
}

type gormProviderRepository struct {
	db *gorm.DB
}

func NewGormProviderRepository(db *gorm.DB) ProviderRepository {
	return &gormProviderRepository{db: db}
}

func (r *gormProviderRepository) UpsertProvider(ctx context.Context, provider domain.Provider) (domain.Provider, error) {
	if provider.ID == "" {
		provider.ID = newID(idPrefixProvider)
	}
	record := providerToRecord(provider)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"type",
			"base_url",
			"auth_type",
			"default_headers",
			"status",
			"proxy_strategy",
			"mode",
			"adapter",
			"api_key",
			"updated_at",
		}),
	}).Create(&record).Error
	return providerFromRecord(record), err
}

func (r *gormProviderRepository) GetProvider(ctx context.Context, id string) (domain.Provider, bool, error) {
	var provider ProviderRecord
	err := r.db.WithContext(ctx).First(&provider, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Provider{}, false, nil
	}
	if err != nil {
		return domain.Provider{}, false, err
	}
	return providerFromRecord(provider), true, nil
}

func (r *gormProviderRepository) ListProviders(ctx context.Context) ([]domain.Provider, error) {
	var providers []ProviderRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&providers).Error
	return providersFromRecords(providers), err
}

func (r *gormProviderRepository) UpsertCredential(ctx context.Context, credential domain.ProviderCredential) error {
	record := credentialToRecord(credential)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider_id",
			"name",
			"secret_ref",
			"status",
			"last_used_at",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *gormProviderRepository) UpsertModelMapping(ctx context.Context, mapping domain.ModelMapping) (domain.ModelMapping, error) {
	if mapping.ID == "" {
		mapping.ID = newID(idPrefixModelMapping)
	}
	record := mappingToRecord(mapping)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider_id",
			"from_model",
			"to_model",
			"updated_at",
		}),
	}).Create(&record).Error
	return mappingFromRecord(record), err
}

func (r *gormProviderRepository) ListModelMappings(ctx context.Context) ([]domain.ModelMapping, error) {
	var mappings []ModelMappingRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&mappings).Error
	return mappingsFromRecords(mappings), err
}

func (r *gormProviderRepository) ListModelMappingsByProvider(ctx context.Context, providerID string) ([]domain.ModelMapping, error) {
	var mappings []ModelMappingRecord
	err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Order("created_at ASC").Find(&mappings).Error
	return mappingsFromRecords(mappings), err
}

func (r *gormProviderRepository) DeleteModelMapping(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&ModelMappingRecord{}).Error
}
