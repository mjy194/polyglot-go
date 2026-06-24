package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AccountRepository persists adapter account sources and managed accounts.
type AccountRepository interface {
	UpsertSource(ctx context.Context, source domain.AccountSource) error
	GetSource(ctx context.Context, sourceID string) (domain.AccountSource, bool, error)
	UpsertAccounts(ctx context.Context, accounts []domain.Account) error
	ListAvailableAccounts(ctx context.Context, provider string, now time.Time) ([]domain.Account, error)
	AcquireAccount(ctx context.Context, provider, sessionID string, now time.Time) (domain.Account, bool, error)
	AcquireAccountByID(ctx context.Context, accountID, sessionID string, now time.Time) (domain.Account, bool, error)
	ReleaseAccount(ctx context.Context, accountID string) error
	RecordUsage(ctx context.Context, event domain.UsageEvent) error
}

type gormAccountRepository struct {
	db *gorm.DB
}

func NewGormAccountRepository(db *gorm.DB) AccountRepository {
	return &gormAccountRepository{db: db}
}

func (r *gormAccountRepository) UpsertSource(ctx context.Context, source domain.AccountSource) error {
	record := sourceToRecord(source)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider",
			"callback_addr",
			"capabilities",
			"watermark",
			"status",
			"last_seen_at",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *gormAccountRepository) GetSource(ctx context.Context, sourceID string) (domain.AccountSource, bool, error) {
	var source AccountSourceRecord
	err := r.db.WithContext(ctx).First(&source, "source_id = ?", sourceID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AccountSource{}, false, nil
	}
	if err != nil {
		return domain.AccountSource{}, false, err
	}
	return sourceFromRecord(source), true, nil
}

func (r *gormAccountRepository) UpsertAccounts(ctx context.Context, accounts []domain.Account) error {
	if len(accounts) == 0 {
		return nil
	}
	records := accountsToRecords(accounts)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_id",
			"provider",
			"credentials",
			"metadata",
			"expires_at",
			"health",
			"updated_at",
		}),
	}).Create(&records).Error
}

func (r *gormAccountRepository) ListAvailableAccounts(ctx context.Context, provider string, now time.Time) ([]domain.Account, error) {
	var accounts []AccountRecord
	err := r.db.WithContext(ctx).
		Where("provider = ? AND health = ? AND in_use = ? AND expires_at > ?", provider, domain.StatusHealthy, false, now).
		Order("usage_count ASC, updated_at ASC").
		Find(&accounts).Error
	return accountsFromRecords(accounts), err
}

func (r *gormAccountRepository) AcquireAccount(ctx context.Context, provider, sessionID string, now time.Time) (domain.Account, bool, error) {
	var acquired AccountRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account AccountRecord
		err := tx.
			Where("provider = ? AND health = ? AND in_use = ? AND expires_at > ?", provider, domain.StatusHealthy, false, now).
			Order("usage_count ASC, updated_at ASC").
			First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err != nil {
			return err
		}

		if err := markAccountAcquired(tx, &account, sessionID, now); err != nil {
			return err
		}
		acquired = account
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return accountFromRecord(acquired), true, nil
}

func (r *gormAccountRepository) AcquireAccountByID(ctx context.Context, accountID, sessionID string, now time.Time) (domain.Account, bool, error) {
	var acquired AccountRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var account AccountRecord
		err := tx.
			Where("account_id = ? AND health = ? AND in_use = ? AND expires_at > ?", accountID, domain.StatusHealthy, false, now).
			First(&account).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err != nil {
			return err
		}

		if err := markAccountAcquired(tx, &account, sessionID, now); err != nil {
			return err
		}
		acquired = account
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Account{}, false, nil
	}
	if err != nil {
		return domain.Account{}, false, err
	}
	return accountFromRecord(acquired), true, nil
}

func (r *gormAccountRepository) ReleaseAccount(ctx context.Context, accountID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AccountRecord{}).
			Where("account_id = ?", accountID).
			Update("in_use", false).Error; err != nil {
			return err
		}
		return tx.Model(&AccountLeaseRecord{}).
			Where("account_id = ? AND status = ?", accountID, domain.LeaseStatusActive).
			Updates(map[string]interface{}{
				"status":      domain.LeaseStatusReleased,
				"released_at": &now,
			}).Error
	})
}

func markAccountAcquired(tx *gorm.DB, account *AccountRecord, sessionID string, now time.Time) error {
	update := map[string]interface{}{
		"in_use":       true,
		"usage_count":  gorm.Expr("usage_count + ?", 1),
		"last_used_at": now,
	}
	res := tx.Model(&AccountRecord{}).
		Where("account_id = ? AND in_use = ?", account.AccountID, false).
		Updates(update)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	lease := AccountLeaseRecord{
		ID:         newLeaseID(),
		AccountID:  account.AccountID,
		SessionID:  sessionID,
		Status:     domain.LeaseStatusActive,
		AcquiredAt: now,
	}
	if err := tx.Create(&lease).Error; err != nil {
		return err
	}
	account.InUse = true
	account.UsageCount++
	account.LastUsedAt = &now
	return nil
}

func (r *gormAccountRepository) RecordUsage(ctx context.Context, event domain.UsageEvent) error {
	record := usageToRecord(event)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		if event.AccountID != "" && event.RequestsCount != 0 {
			if err := tx.Model(&AccountRecord{}).
				Where("account_id = ?", event.AccountID).
				Update("usage_count", gorm.Expr("usage_count + ?", event.RequestsCount)).
				Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func newLeaseID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return "lease_" + hex.EncodeToString(b[:])
	}
	return "lease_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
