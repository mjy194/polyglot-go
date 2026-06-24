package data

import (
	"fmt"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"

	defaultSQLiteDSN = "data.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
)

// Config describes the database backend for the data service.
type Config struct {
	Driver      string
	DSN         string
	AutoMigrate bool
}

// Store owns the GORM database handle and repository implementations.
type Store struct {
	db        *gorm.DB
	authState AuthStateRepository
	kvStore   KVStoreRepository
	accounts  AccountRepository
	providers ProviderRepository
	proxies   ProxyRepository
	identity  IdentityRepository
	adapters  AdapterRepository
	audit     AuditRepository
}

// Open connects to the configured database and initializes repositories.
func Open(cfg Config) (*Store, error) {
	cfg = normalizeConfig(cfg)

	dialector, err := dialector(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if cfg.AutoMigrate {
		if err := db.AutoMigrate(migrationModels()...); err != nil {
			return nil, fmt.Errorf("auto migrate database: %w", err)
		}
		if err := repairLegacyOrgColumns(db); err != nil {
			return nil, fmt.Errorf("repair legacy org columns: %w", err)
		}
		if err := repairProxiesLegacyType(db); err != nil {
			return nil, fmt.Errorf("repair legacy proxies.type column: %w", err)
		}
		if err := repairBlankPrimaryKeys(db); err != nil {
			return nil, fmt.Errorf("repair blank primary keys: %w", err)
		}
	}

	return &Store{
		db:        db,
		authState: NewGormAuthStateRepository(db),
		kvStore:   NewGormKVStoreRepository(db),
		accounts:  NewGormAccountRepository(db),
		providers: NewGormProviderRepository(db),
		proxies:   NewGormProxyRepository(db),
		identity:  NewGormIdentityRepository(db),
		adapters:  NewGormAdapterRepository(db),
		audit:     NewGormAuditRepository(db),
	}, nil
}

func migrationModels() []interface{} {
	return []interface{}{
		&KVStoreRecord{},
		&AuthStateRecord{},
		&UserRecord{},
		&RoleRecord{},
		&UserRoleRecord{},
		&APIKeyRecord{},
		&AdminSessionRecord{},
		&ProviderRecord{},
		&ProxyRecord{},
		&ProviderProxyRecord{},
		&ProviderCredentialRecord{},
		&ModelMappingRecord{},
		&AdapterRecord{},
		&AdapterInstanceRecord{},
		&AccountSourceRecord{},
		&AccountRecord{},
		&AccountLeaseRecord{},
		&UsageEventRecord{},
		&RequestLogRecord{},
	}
}

func normalizeConfig(cfg Config) Config {
	cfg.Driver = strings.ToLower(strings.TrimSpace(cfg.Driver))
	if cfg.Driver == "" {
		cfg.Driver = DriverSQLite
	}
	if cfg.DSN == "" && cfg.Driver == DriverSQLite {
		cfg.DSN = defaultSQLiteDSN
	}
	return cfg
}

func dialector(cfg Config) (gorm.Dialector, error) {
	switch cfg.Driver {
	case DriverSQLite:
		return sqlite.Open(SQLiteDSN(cfg.DSN)), nil
	case DriverPostgres, "postgresql":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("postgres dsn is required")
		}
		return postgres.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

// SQLiteDSN adds sane defaults for direct SQLite file paths while preserving
// explicit query parameters such as in-memory databases.
func SQLiteDSN(dsn string) string {
	if dsn == "" {
		return defaultSQLiteDSN
	}
	if strings.Contains(dsn, "?") || strings.HasPrefix(dsn, "file:") || dsn == ":memory:" {
		return dsn
	}
	return dsn + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
}

// DB exposes the underlying GORM handle for tests and advanced migrations.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// AuthStates returns the repository for key/value auth state rows.
func (s *Store) AuthStates() AuthStateRepository {
	return s.authState
}

// KVStore returns the repository for generic adapter key/value rows.
func (s *Store) KVStore() KVStoreRepository {
	return s.kvStore
}

// Accounts returns account source/account/lease persistence operations.
func (s *Store) Accounts() AccountRepository {
	return s.accounts
}

// Providers returns direct provider persistence operations.
func (s *Store) Providers() ProviderRepository {
	return s.providers
}

// Proxies returns network proxy persistence operations.
func (s *Store) Proxies() ProxyRepository {
	return s.proxies
}

// Identity returns tenant, user, role, and API key persistence operations.
func (s *Store) Identity() IdentityRepository {
	return s.identity
}

// Adapters returns adapter definition and instance persistence operations.
func (s *Store) Adapters() AdapterRepository {
	return s.adapters
}

// Audit returns request and usage audit persistence operations.
func (s *Store) Audit() AuditRepository {
	return s.audit
}

// Close closes the underlying sql.DB connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
