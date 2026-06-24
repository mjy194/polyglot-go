package data

import (
	"context"
	"errors"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProxyRepository persists network proxies and the provider↔proxy associations.
type ProxyRepository interface {
	UpsertProxy(ctx context.Context, proxy domain.Proxy) (domain.Proxy, error)
	GetProxy(ctx context.Context, id string) (domain.Proxy, bool, error)
	ListProxies(ctx context.Context) ([]domain.Proxy, error)
	DeleteProxy(ctx context.Context, id string) error

	// ListProviderProxies returns the proxies attached to a provider, ordered by
	// priority ascending (join + proxy details). Excludes disabled proxies.
	ListProviderProxies(ctx context.Context, providerID string) ([]domain.ProviderProxy, error)
	// SetProviderProxies replaces all associations for a provider in one transaction.
	SetProviderProxies(ctx context.Context, providerID string, associations []domain.ProviderProxy) error
}

type gormProxyRepository struct {
	db *gorm.DB
}

func NewGormProxyRepository(db *gorm.DB) ProxyRepository {
	return &gormProxyRepository{db: db}
}

func (r *gormProxyRepository) UpsertProxy(ctx context.Context, proxy domain.Proxy) (domain.Proxy, error) {
	if proxy.ID == "" {
		proxy.ID = newID(idPrefixProxy)
	}
	record := proxyToRecord(proxy)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"url",
			"username",
			"password",
			"status",
			"updated_at",
		}),
	}).Create(&record).Error
	return proxyFromRecord(record), err
}

func (r *gormProxyRepository) GetProxy(ctx context.Context, id string) (domain.Proxy, bool, error) {
	var record ProxyRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Proxy{}, false, nil
	}
	if err != nil {
		return domain.Proxy{}, false, err
	}
	return proxyFromRecord(record), true, nil
}

func (r *gormProxyRepository) ListProxies(ctx context.Context) ([]domain.Proxy, error) {
	var records []ProxyRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return proxiesFromRecords(records), err
}

func (r *gormProxyRepository) DeleteProxy(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&ProxyRecord{}).Error
}

func (r *gormProxyRepository) ListProviderProxies(ctx context.Context, providerID string) ([]domain.ProviderProxy, error) {
	var records []ProviderProxyRecord
	err := r.db.WithContext(ctx).
		Where("provider_id = ?", providerID).
		Order("priority ASC").
		Find(&records).Error
	return providerProxiesFromRecords(records), err
}

func (r *gormProxyRepository) SetProviderProxies(ctx context.Context, providerID string, associations []domain.ProviderProxy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", providerID).Delete(&ProviderProxyRecord{}).Error; err != nil {
			return err
		}
		if len(associations) == 0 {
			return nil
		}
		records := make([]ProviderProxyRecord, 0, len(associations))
		for _, a := range associations {
			a.ProviderID = providerID
			records = append(records, providerProxyToRecord(a))
		}
		return tx.Create(&records).Error
	})
}
