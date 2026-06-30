package data

import (
	"context"
	"errors"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GroupRepository persists groups and the group↔provider associations.
type GroupRepository interface {
	UpsertGroup(ctx context.Context, group domain.Group) (domain.Group, error)
	GetGroup(ctx context.Context, id string) (domain.Group, bool, error)
	GetGroupByName(ctx context.Context, name string) (domain.Group, bool, error)
	ListGroups(ctx context.Context) ([]domain.Group, error)
	DeleteGroup(ctx context.Context, id string) error

	// ListGroupProviders returns the providers attached to a group, ordered by
	// priority ascending.
	ListGroupProviders(ctx context.Context, groupID string) ([]domain.GroupProvider, error)
	// SetGroupProviders replaces all associations for a group in one transaction.
	SetGroupProviders(ctx context.Context, groupID string, associations []domain.GroupProvider) error
	// SetProviderGroups replaces all group memberships for a provider (provider-side edit).
	SetProviderGroups(ctx context.Context, providerID string, groups []domain.GroupProvider) error
	// ListProviderGroups returns the groups a provider belongs to.
	ListProviderGroups(ctx context.Context, providerID string) ([]domain.GroupProvider, error)
	// ListProvidersForGroup returns active providers in a group (for routing).
	ListProvidersForGroup(ctx context.Context, groupID string) ([]domain.Provider, error)
}

type gormGroupRepository struct {
	db *gorm.DB
}

func NewGormGroupRepository(db *gorm.DB) GroupRepository {
	return &gormGroupRepository{db: db}
}

func (r *gormGroupRepository) UpsertGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	if group.ID == "" {
		group.ID = newID(idPrefixGroup)
	}
	if group.Strategy == "" {
		group.Strategy = "failover"
	}
	if group.Status == "" {
		group.Status = domain.StatusActive
	}
	record := groupToRecord(group)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"description",
			"ratio",
			"strategy",
			"status",
			"updated_at",
		}),
	}).Create(&record).Error
	return groupFromRecord(record), err
}

func (r *gormGroupRepository) GetGroup(ctx context.Context, id string) (domain.Group, bool, error) {
	var record GroupRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Group{}, false, nil
	}
	if err != nil {
		return domain.Group{}, false, err
	}
	return groupFromRecord(record), true, nil
}

func (r *gormGroupRepository) GetGroupByName(ctx context.Context, name string) (domain.Group, bool, error) {
	var record GroupRecord
	err := r.db.WithContext(ctx).First(&record, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Group{}, false, nil
	}
	if err != nil {
		return domain.Group{}, false, err
	}
	return groupFromRecord(record), true, nil
}

func (r *gormGroupRepository) ListGroups(ctx context.Context) ([]domain.Group, error) {
	var records []GroupRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return groupsFromRecords(records), err
}

func (r *gormGroupRepository) DeleteGroup(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", id).Delete(&GroupProviderRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&GroupRecord{}).Error
	})
}

func (r *gormGroupRepository) ListGroupProviders(ctx context.Context, groupID string) ([]domain.GroupProvider, error) {
	var records []GroupProviderRecord
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Order("priority ASC").Find(&records).Error
	return groupProvidersFromRecords(records), err
}

func (r *gormGroupRepository) ListProviderGroups(ctx context.Context, providerID string) ([]domain.GroupProvider, error) {
	var records []GroupProviderRecord
	err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Order("priority ASC").Find(&records).Error
	return groupProvidersFromRecords(records), err
}

func (r *gormGroupRepository) SetGroupProviders(ctx context.Context, groupID string, associations []domain.GroupProvider) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&GroupProviderRecord{}).Error; err != nil {
			return err
		}
		records := make([]GroupProviderRecord, 0, len(associations))
		for _, a := range associations {
			a.GroupID = groupID
			records = append(records, groupProviderToRecord(a))
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Create(&records).Error
	})
}

func (r *gormGroupRepository) SetProviderGroups(ctx context.Context, providerID string, groups []domain.GroupProvider) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", providerID).Delete(&GroupProviderRecord{}).Error; err != nil {
			return err
		}
		records := make([]GroupProviderRecord, 0, len(groups))
		for _, g := range groups {
			g.ProviderID = providerID
			records = append(records, groupProviderToRecord(g))
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Create(&records).Error
	})
}

// ListProvidersForGroup returns active providers in priority order for routing.
func (r *gormGroupRepository) ListProvidersForGroup(ctx context.Context, groupID string) ([]domain.Provider, error) {
	var links []GroupProviderRecord
	if err := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("priority ASC").
		Find(&links).Error; err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.ProviderID)
	}
	var records []ProviderRecord
	if err := r.db.WithContext(ctx).Where("id IN ? AND status = ?", ids, domain.StatusActive).Find(&records).Error; err != nil {
		return nil, err
	}

	byID := make(map[string]ProviderRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}
	out := make([]domain.Provider, 0, len(records))
	for _, link := range links {
		if record, ok := byID[link.ProviderID]; ok {
			out = append(out, providerFromRecord(record))
		}
	}
	return out, nil
}
