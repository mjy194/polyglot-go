package data

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"polyglot/internal/domain"
)

func TestAuthStateRepositoryUpsertsAndLoadsRecord(t *testing.T) {
	store, err := Open(Config{
		Driver:      DriverSQLite,
		DSN:         filepath.Join(t.TempDir(), "data.db"),
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo := store.AuthStates()

	if _, found, err := repo.Get(ctx, "default"); err != nil || found {
		t.Fatalf("empty Get found=%v err=%v", found, err)
	}

	if err := repo.Upsert(ctx, AuthStateRecord{Key: "default", Value: `{"v":1}`, UpdatedAt: 1}); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}
	if err := repo.Upsert(ctx, AuthStateRecord{Key: "default", Value: `{"v":2}`, UpdatedAt: 2}); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	got, found, err := repo.Get(ctx, "default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatalf("expected record to be found")
	}
	if got.Value != `{"v":2}` || got.UpdatedAt != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestStoreMigratesFirstPhaseTables(t *testing.T) {
	store, err := Open(Config{
		Driver:      DriverSQLite,
		DSN:         filepath.Join(t.TempDir(), "data.db"),
		AutoMigrate: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	for _, model := range []interface{}{
		&UserRecord{},
		&RoleRecord{},
		&APIKeyRecord{},
		&ProviderRecord{},
		&ProviderCredentialRecord{},
		&ModelMappingRecord{},
		&AdapterRecord{},
		&AdapterInstanceRecord{},
		&AccountSourceRecord{},
		&AccountRecord{},
		&AccountLeaseRecord{},
		&UsageEventRecord{},
		&RequestLogRecord{},
	} {
		if !store.DB().Migrator().HasTable(model) {
			t.Fatalf("missing migrated table for %T", model)
		}
	}
}

func TestProviderRepositoryUpsertsProviderAndMappings(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo := store.Providers()
	provider := domain.Provider{
		ID:             "prov_openai",
		Name:           "OpenAI",
		Type:           "openai",
		BaseURL:        "https://api.openai.com",
		AuthType:       "bearer",
		DefaultHeaders: "{}",
		Status:         domain.StatusActive,
	}
	if _, err := repo.UpsertProvider(ctx, provider); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}
	provider.Name = "OpenAI Direct"
	if _, err := repo.UpsertProvider(ctx, provider); err != nil {
		t.Fatalf("second UpsertProvider: %v", err)
	}

	got, found, err := repo.GetProvider(ctx, "prov_openai")
	if err != nil || !found {
		t.Fatalf("GetProvider found=%v err=%v", found, err)
	}
	if got.Name != "OpenAI Direct" {
		t.Fatalf("provider not updated: %+v", got)
	}

	if err := repo.UpsertCredential(ctx, domain.ProviderCredential{
		ID:         "cred_1",
		ProviderID: "prov_openai",
		Name:       "primary",
		SecretRef:  "secret/provider/openai/primary",
		Status:     domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertCredential: %v", err)
	}

	if _, err := repo.UpsertModelMapping(ctx, domain.ModelMapping{
		ID:         "map_1",
		ProviderID: "prov_openai",
		FromModel:  "gpt-latest",
		ToModel:    "gpt-4.1",
	}); err != nil {
		t.Fatalf("UpsertModelMapping: %v", err)
	}
	mappings, err := repo.ListModelMappings(ctx)
	if err != nil {
		t.Fatalf("ListModelMappings: %v", err)
	}
	if len(mappings) != 1 || mappings[0].ToModel != "gpt-4.1" {
		t.Fatalf("unexpected mappings: %+v", mappings)
	}
}

func TestIdentityRepositoryLifecycle(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo := store.Identity()

	if _, err := repo.UpsertUser(ctx, domain.User{
		ID:          "user_1",
		Email:       "a@example.com",
		DisplayName: "Alice",
		Status:      domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if _, err := repo.UpsertRole(ctx, domain.Role{
		ID:          "role_admin",
		Name:        "admin",
		Permissions: `["*"]`,
	}); err != nil {
		t.Fatalf("UpsertRole: %v", err)
	}
	if err := repo.AssignRole(ctx, domain.UserRole{UserID: "user_1", RoleID: "role_admin"}); err != nil {
		t.Fatalf("AssignRole: %v", err)
	}
	userRoles, err := repo.ListUserRoles(ctx, "user_1")
	if err != nil {
		t.Fatalf("ListUserRoles: %v", err)
	}
	if len(userRoles) != 1 || userRoles[0].RoleID != "role_admin" {
		t.Fatalf("unexpected roles: %+v", userRoles)
	}

	if _, err := repo.UpsertAPIKey(ctx, domain.APIKey{
		ID:     "key_1",
		UserID: "user_1",
		Name:   "primary",
		Key:    "pk_test",
		Scopes: `["chat"]`,
		Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertAPIKey: %v", err)
	}
	key, found, err := repo.GetAPIKeyByKey(ctx, "pk_test")
	if err != nil || !found {
		t.Fatalf("GetAPIKeyByKey found=%v err=%v", found, err)
	}
	if key.UserID != "user_1" {
		t.Fatalf("unexpected api key: %+v", key)
	}
}

func TestStoreDropsLegacyAdminSessionOrgIDColumn(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "data.db")
	legacy, err := Open(Config{Driver: DriverSQLite, DSN: dsn, AutoMigrate: false})
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := legacy.DB().Exec(`
		CREATE TABLE admin_sessions (
			id text PRIMARY KEY,
			user_id text NOT NULL,
			org_id text NOT NULL,
			token_hash text NOT NULL,
			scopes text NOT NULL DEFAULT '[]',
			status text NOT NULL DEFAULT 'active',
			expires_at datetime,
			created_at datetime,
			updated_at datetime
		)
	`).Error; err != nil {
		t.Fatalf("create legacy admin_sessions: %v", err)
	}
	if err := legacy.DB().Exec(`CREATE INDEX idx_admin_sessions_org_id ON admin_sessions(org_id)`).Error; err != nil {
		t.Fatalf("create legacy org index: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := Open(Config{Driver: DriverSQLite, DSN: dsn, AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open migrated db: %v", err)
	}
	defer store.Close()

	hasColumn, err := sqliteColumnExists(store.DB(), "admin_sessions", "org_id")
	if err != nil {
		t.Fatalf("inspect admin_sessions: %v", err)
	}
	if hasColumn {
		t.Fatalf("legacy org_id column was not removed")
	}

	if _, err := store.Identity().CreateAdminSession(context.Background(), domain.AdminSession{
		UserID:    "user_1",
		TokenHash: "hash_1",
		Scopes:    `["admin"]`,
		Status:    domain.StatusActive,
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateAdminSession after migration: %v", err)
	}
}

func TestRepairBlankAdminPrimaryKeys(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	if err := store.DB().Create(&ProviderRecord{
		ID:             "",
		Name:           "Legacy Provider",
		Type:           "openai",
		BaseURL:        "https://api.openai.com",
		AuthType:       "bearer",
		DefaultHeaders: "{}",
		Status:         domain.StatusActive,
	}).Error; err != nil {
		t.Fatalf("create legacy provider: %v", err)
	}
	if err := store.DB().Create(&APIKeyRecord{
		ID:     "",
		Name:   "legacy key",
		Key:    "pk_legacy",
		Scopes: `["admin"]`,
		Status: domain.StatusActive,
	}).Error; err != nil {
		t.Fatalf("create legacy api key: %v", err)
	}

	if err := repairBlankPrimaryKeys(store.DB()); err != nil {
		t.Fatalf("repairBlankPrimaryKeys: %v", err)
	}

	providers, err := store.Providers().ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(providers) != 1 || providers[0].ID == "" {
		t.Fatalf("provider id was not repaired: %+v", providers)
	}

	keys, err := store.Identity().ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID == "" {
		t.Fatalf("api key id was not repaired: %+v", keys)
	}
}

func TestAdapterRepositoryLifecycle(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo := store.Adapters()
	now := time.Now().UTC()

	if err := repo.UpsertAdapter(ctx, domain.Adapter{
		ID:     "adapter_uipath",
		Name:   "UiPath",
		Type:   "uipath",
		Status: domain.StatusActive,
	}); err != nil {
		t.Fatalf("UpsertAdapter: %v", err)
	}
	if err := repo.UpsertInstance(ctx, domain.AdapterInstance{
		ID:              "inst_1",
		AdapterID:       "adapter_uipath",
		Provider:        "uipath",
		CallbackAddr:    "127.0.0.1:50051",
		Capabilities:    `["chat"]`,
		Metadata:        `{}`,
		Status:          domain.StatusActive,
		LastHeartbeatAt: &now,
	}); err != nil {
		t.Fatalf("UpsertInstance: %v", err)
	}
	later := now.Add(time.Minute)
	if err := repo.MarkHeartbeat(ctx, "inst_1", later); err != nil {
		t.Fatalf("MarkHeartbeat: %v", err)
	}
	instance, found, err := repo.GetInstance(ctx, "inst_1")
	if err != nil || !found {
		t.Fatalf("GetInstance found=%v err=%v", found, err)
	}
	if instance.LastHeartbeatAt == nil || !instance.LastHeartbeatAt.Equal(later) {
		t.Fatalf("unexpected heartbeat: %+v", instance)
	}
	if err := repo.SetInstanceStatus(ctx, "inst_1", domain.StatusDisabled); err != nil {
		t.Fatalf("SetInstanceStatus: %v", err)
	}
	instances, err := repo.ListInstances(ctx, "adapter_uipath")
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 1 || instances[0].Status != domain.StatusDisabled {
		t.Fatalf("unexpected instances: %+v", instances)
	}
}

func TestAccountRepositoryLifecycle(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	repo := store.Accounts()
	now := time.Now().UTC()

	source := domain.AccountSource{
		SourceID:     "src_1",
		Provider:     "uipath",
		CallbackAddr: "127.0.0.1:50051",
		Capabilities: `["supply"]`,
		Watermark:    `{"low":1}`,
		Status:       domain.StatusActive,
		LastSeenAt:   &now,
	}
	if err := repo.UpsertSource(ctx, source); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}
	if _, found, err := repo.GetSource(ctx, "src_1"); err != nil || !found {
		t.Fatalf("GetSource found=%v err=%v", found, err)
	}

	if err := repo.UpsertAccounts(ctx, []domain.Account{
		{
			AccountID:   "acc_1",
			SourceID:    "src_1",
			Provider:    "uipath",
			Credentials: `{"access_token":"a"}`,
			Metadata:    `{"email":"a@example.com"}`,
			ExpiresAt:   now.Add(time.Hour),
			Health:      domain.StatusHealthy,
		},
	}); err != nil {
		t.Fatalf("UpsertAccounts: %v", err)
	}

	available, err := repo.ListAvailableAccounts(ctx, "uipath", now)
	if err != nil {
		t.Fatalf("ListAvailableAccounts: %v", err)
	}
	if len(available) != 1 {
		t.Fatalf("available=%d, want 1", len(available))
	}

	acquired, ok, err := repo.AcquireAccount(ctx, "uipath", "session_1", now)
	if err != nil || !ok {
		t.Fatalf("AcquireAccount ok=%v err=%v", ok, err)
	}
	if acquired.AccountID != "acc_1" || !acquired.InUse {
		t.Fatalf("unexpected acquired account: %+v", acquired)
	}
	var lease AccountLeaseRecord
	if err := store.DB().First(&lease, "account_id = ? AND session_id = ?", "acc_1", "session_1").Error; err != nil {
		t.Fatalf("load lease: %v", err)
	}
	if lease.Status != domain.LeaseStatusActive || lease.AcquiredAt.IsZero() {
		t.Fatalf("unexpected lease after acquire: %+v", lease)
	}
	if _, ok, err := repo.AcquireAccount(ctx, "uipath", "session_2", now); err != nil || ok {
		t.Fatalf("second AcquireAccount ok=%v err=%v, want unavailable", ok, err)
	}

	if err := repo.ReleaseAccount(ctx, "acc_1"); err != nil {
		t.Fatalf("ReleaseAccount: %v", err)
	}
	if err := store.DB().First(&lease, "id = ?", lease.ID).Error; err != nil {
		t.Fatalf("reload lease: %v", err)
	}
	if lease.Status != domain.LeaseStatusReleased || lease.ReleasedAt == nil {
		t.Fatalf("unexpected lease after release: %+v", lease)
	}
	if _, ok, err := repo.AcquireAccount(ctx, "uipath", "session_3", now); err != nil || !ok {
		t.Fatalf("Acquire after release ok=%v err=%v", ok, err)
	}

	if err := repo.RecordUsage(ctx, domain.UsageEvent{
		ID:            "usage_1",
		AccountID:     "acc_1",
		Provider:      "uipath",
		Model:         "claude-opus-4-8",
		TokensUsed:    42,
		RequestsCount: 1,
	}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	var stored AccountRecord
	if err := store.DB().First(&stored, "account_id = ?", "acc_1").Error; err != nil {
		t.Fatalf("load account after usage: %v", err)
	}
	if stored.UsageCount != 3 {
		t.Fatalf("UsageCount=%d, want 3", stored.UsageCount)
	}
}

func TestAuditRepositoryRecordsAndFiltersEvents(t *testing.T) {
	store, err := Open(Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "data.db"), AutoMigrate: true})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if err := store.Audit().RecordRequest(ctx, domain.RequestLog{
		ID:         "req_1",
		UserID:     "user_1",
		Provider:   "openai",
		Protocol:   "openai",
		Model:      "gpt-4.1",
		StatusCode: 200,
		Success:    true,
		LatencyMs:  123,
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("RecordRequest: %v", err)
	}
	logs, err := store.Audit().ListRequestLogs(ctx, RequestLogFilter{
		Provider: "openai",
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("ListRequestLogs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != "req_1" {
		t.Fatalf("unexpected logs: %+v", logs)
	}

	if err := store.Accounts().RecordUsage(ctx, domain.UsageEvent{
		ID:            "usage_audit_1",
		UserID:        "user_1",
		Provider:      "openai",
		Model:         "gpt-4.1",
		TokensUsed:    10,
		RequestsCount: 1,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	events, err := store.Audit().ListUsageEvents(ctx, UsageEventFilter{
		Model: "gpt-4.1",
	})
	if err != nil {
		t.Fatalf("ListUsageEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != "usage_audit_1" {
		t.Fatalf("unexpected usage events: %+v", events)
	}
}

func TestSQLiteDSNAddsDefaultsToPlainPath(t *testing.T) {
	got := SQLiteDSN("data.db")
	if got == "data.db" {
		t.Fatalf("SQLiteDSN should add query defaults")
	}
	if got := SQLiteDSN("file::memory:?cache=shared"); got != "file::memory:?cache=shared" {
		t.Fatalf("explicit DSN changed: %q", got)
	}
}
