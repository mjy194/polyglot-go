package data

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

var legacyOrgColumnTables = []string{
	"roles",
	"api_keys",
	"providers",
	"model_mappings",
	"adapters",
	"usage_events",
	"request_logs",
	"admin_sessions",
}

func repairLegacyOrgColumns(db *gorm.DB) error {
	if db.Dialector.Name() != DriverSQLite {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, table := range legacyOrgColumnTables {
			hasColumn, err := sqliteColumnExists(tx, table, "org_id")
			if err != nil {
				return err
			}
			if !hasColumn {
				continue
			}
			if err := dropSQLiteIndexesForColumn(tx, table, "org_id"); err != nil {
				return err
			}
			if err := tx.Exec(
				"ALTER TABLE " + quoteSQLiteIdent(table) + " DROP COLUMN " + quoteSQLiteIdent("org_id"),
			).Error; err != nil {
				return fmt.Errorf("drop legacy org_id from %s: %w", table, err)
			}
		}
		return nil
	})
}

type sqliteColumnInfo struct {
	Name string `gorm:"column:name"`
}

func sqliteColumnExists(tx *gorm.DB, table, column string) (bool, error) {
	var columns []sqliteColumnInfo
	if err := tx.Raw("PRAGMA table_info(" + quoteSQLiteIdent(table) + ")").Scan(&columns).Error; err != nil {
		return false, fmt.Errorf("inspect columns for %s: %w", table, err)
	}
	for _, info := range columns {
		if strings.EqualFold(info.Name, column) {
			return true, nil
		}
	}
	return false, nil
}

type sqliteIndexListInfo struct {
	Name string `gorm:"column:name"`
}

type sqliteIndexColumnInfo struct {
	Name string `gorm:"column:name"`
}

func dropSQLiteIndexesForColumn(tx *gorm.DB, table, column string) error {
	var indexes []sqliteIndexListInfo
	if err := tx.Raw("PRAGMA index_list(" + quoteSQLiteIdent(table) + ")").Scan(&indexes).Error; err != nil {
		return fmt.Errorf("inspect indexes for %s: %w", table, err)
	}
	for _, index := range indexes {
		if strings.HasPrefix(index.Name, "sqlite_autoindex_") {
			continue
		}
		usesColumn, err := sqliteIndexUsesColumn(tx, index.Name, column)
		if err != nil {
			return err
		}
		if usesColumn {
			if err := tx.Exec("DROP INDEX IF EXISTS " + quoteSQLiteIdent(index.Name)).Error; err != nil {
				return fmt.Errorf("drop index %s: %w", index.Name, err)
			}
		}
	}
	return nil
}

func sqliteIndexUsesColumn(tx *gorm.DB, index, column string) (bool, error) {
	var columns []sqliteIndexColumnInfo
	if err := tx.Raw("PRAGMA index_info(" + quoteSQLiteIdent(index) + ")").Scan(&columns).Error; err != nil {
		return false, fmt.Errorf("inspect index %s: %w", index, err)
	}
	for _, info := range columns {
		if strings.EqualFold(info.Name, column) {
			return true, nil
		}
	}
	return false, nil
}

func quoteSQLiteIdent(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func repairBlankPrimaryKeys(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := repairBlankUserID(tx); err != nil {
			return err
		}
		if err := repairBlankRoleID(tx); err != nil {
			return err
		}
		if err := repairBlankAPIKeyID(tx); err != nil {
			return err
		}
		if err := repairBlankProviderID(tx); err != nil {
			return err
		}
		return repairBlankModelMappingID(tx)
	})
}

func repairBlankUserID(tx *gorm.DB) error {
	id := newID(idPrefixUser)
	res := tx.Model(&UserRecord{}).Where("id = ?", "").Update("id", id)
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	return tx.Model(&UserRoleRecord{}).Where("user_id = ?", "").Update("user_id", id).Error
}

func repairBlankRoleID(tx *gorm.DB) error {
	id := newID(idPrefixRole)
	res := tx.Model(&RoleRecord{}).Where("id = ?", "").Update("id", id)
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	return tx.Model(&UserRoleRecord{}).Where("role_id = ?", "").Update("role_id", id).Error
}

func repairBlankAPIKeyID(tx *gorm.DB) error {
	return repairBlankID(tx, &APIKeyRecord{}, idPrefixAPIKey)
}

func repairBlankProviderID(tx *gorm.DB) error {
	id := newID(idPrefixProvider)
	res := tx.Model(&ProviderRecord{}).Where("id = ?", "").Update("id", id)
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error
	}
	return tx.Model(&ProviderCredentialRecord{}).Where("provider_id = ?", "").Update("provider_id", id).Error
}

func repairBlankModelMappingID(tx *gorm.DB) error {
	return repairBlankID(tx, &ModelMappingRecord{}, idPrefixModelMapping)
}

func repairBlankID(tx *gorm.DB, model interface{}, prefix string) error {
	res := tx.Model(model).Where("id = ?", "").Update("id", newID(prefix))
	if res.Error != nil {
		return res.Error
	}
	return nil
}
