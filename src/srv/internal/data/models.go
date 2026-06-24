package data

import (
	"time"

	"polyglot/internal/domain"
)

// User is a login principal.
type UserRecord struct {
	ID           string `gorm:"primaryKey;size:64"`
	Email        string `gorm:"size:320;uniqueIndex;not null"`
	DisplayName  string `gorm:"size:255"`
	PasswordHash string `gorm:"size:255"`
	Status       string `gorm:"size:32;not null;default:active"`
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Role groups permissions for users.
type RoleRecord struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	Permissions string `gorm:"type:text;not null;default:'[]'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserRole assigns a role to a user.
type UserRoleRecord struct {
	UserID string `gorm:"primaryKey;size:64"`
	RoleID string `gorm:"primaryKey;size:64"`
}

// APIKey is an inbound platform credential.
type APIKeyRecord struct {
	ID         string `gorm:"primaryKey;size:64"`
	UserID     string `gorm:"size:64;index"`
	Name       string `gorm:"size:255;not null"`
	Key        string `gorm:"size:255;uniqueIndex;not null"`
	Scopes     string `gorm:"type:text;not null;default:'[]'"`
	Status     string `gorm:"size:32;not null;default:active"`
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AdminSession is a password-login session for the administrative API.
type AdminSessionRecord struct {
	ID        string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"size:64;index;not null"`
	TokenHash string `gorm:"size:64;uniqueIndex;not null"`
	Scopes    string `gorm:"type:text;not null;default:'[]'"`
	Status    string `gorm:"size:32;not null;default:active"`
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Provider is a direct upstream provider such as OpenAI, Anthropic, Gemini, or
// an OpenAI-compatible endpoint.
type ProviderRecord struct {
	ID             string `gorm:"primaryKey;size:64"`
	Name           string `gorm:"size:128;not null"`
	Type           string `gorm:"size:64;index;not null"`
	BaseURL        string `gorm:"size:1024;not null"`
	AuthType       string `gorm:"size:64"`
	DefaultHeaders string `gorm:"type:text;not null;default:'{}'"`
	Status         string `gorm:"size:32;not null;default:active"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ProviderCredential stores encrypted direct-provider credentials.
type ProviderCredentialRecord struct {
	ID         string `gorm:"primaryKey;size:64"`
	ProviderID string `gorm:"size:64;index;not null"`
	Name       string `gorm:"size:255;not null"`
	SecretRef  string `gorm:"size:255;not null"`
	Status     string `gorm:"size:32;not null;default:active"`
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ModelMapping maps client-facing model aliases to upstream model names.
type ModelMappingRecord struct {
	ID         string `gorm:"primaryKey;size:64"`
	ProviderID string `gorm:"size:64;index"`
	FromModel  string `gorm:"size:255;not null;index"`
	ToModel    string `gorm:"size:255;not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Adapter is an adapter kind, for example "uipath".
type AdapterRecord struct {
	ID        string `gorm:"primaryKey;size:64"`
	Name      string `gorm:"size:128;not null"`
	Type      string `gorm:"size:64;index;not null"`
	Status    string `gorm:"size:32;not null;default:active"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AdapterInstance is a live adapter process registered with the main service.
type AdapterInstanceRecord struct {
	ID              string `gorm:"primaryKey;size:64"`
	AdapterID       string `gorm:"size:64;index;not null"`
	Provider        string `gorm:"size:64;index;not null"`
	CallbackAddr    string `gorm:"size:512;not null"`
	Capabilities    string `gorm:"type:text;not null;default:'[]'"`
	Metadata        string `gorm:"type:text;not null;default:'{}'"`
	Status          string `gorm:"size:32;not null;default:active"`
	LastHeartbeatAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AccountSource is an adapter-owned source of accounts.
type AccountSourceRecord struct {
	SourceID     string `gorm:"primaryKey;size:128"`
	Provider     string `gorm:"size:64;index;not null"`
	CallbackAddr string `gorm:"size:512;not null"`
	Capabilities string `gorm:"type:text;not null;default:'[]'"`
	Watermark    string `gorm:"type:text;not null;default:'{}'"`
	Status       string `gorm:"size:32;not null;default:active"`
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AccountRecord is a managed account supplied by an adapter.
type AccountRecord struct {
	AccountID   string `gorm:"primaryKey;size:128"`
	SourceID    string `gorm:"size:128;index;not null"`
	Provider    string `gorm:"size:64;index;not null"`
	Credentials string `gorm:"type:text;not null;default:'{}'"`
	Metadata    string `gorm:"type:text;not null;default:'{}'"`
	ExpiresAt   time.Time
	Health      string `gorm:"size:32;index;not null;default:healthy"`
	InUse       bool   `gorm:"index;not null;default:false"`
	UsageCount  int64  `gorm:"not null;default:0"`
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AccountLease records account acquisition lifecycle.
type AccountLeaseRecord struct {
	ID         string `gorm:"primaryKey;size:64"`
	AccountID  string `gorm:"size:128;index;not null"`
	SessionID  string `gorm:"size:255;index"`
	RequestID  string `gorm:"size:255;index"`
	Status     string `gorm:"size:32;index;not null;default:active"`
	AcquiredAt time.Time
	ReleasedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UsageEvent is an append-only usage accounting row.
type UsageEventRecord struct {
	ID            string `gorm:"primaryKey;size:64"`
	UserID        string `gorm:"size:64;index"`
	AccountID     string `gorm:"size:128;index"`
	Provider      string `gorm:"size:64;index"`
	Model         string `gorm:"size:255;index"`
	TokensUsed    int64  `gorm:"not null;default:0"`
	RequestsCount int64  `gorm:"not null;default:0"`
	CreatedAt     time.Time
}

// RequestLog captures request metadata for audit and operations.
type RequestLogRecord struct {
	ID           string `gorm:"primaryKey;size:64"`
	UserID       string `gorm:"size:64;index"`
	APIKeyID     string `gorm:"size:64;index"`
	Provider     string `gorm:"size:64;index"`
	AdapterID    string `gorm:"size:64;index"`
	Protocol     string `gorm:"size:64;index"`
	Model        string `gorm:"size:255;index"`
	StatusCode   int
	Success      bool `gorm:"index"`
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	ErrorType    string `gorm:"size:128"`
	ErrorMessage string `gorm:"type:text"`
	CreatedAt    time.Time
}

func (UserRecord) TableName() string               { return "users" }
func (RoleRecord) TableName() string               { return "roles" }
func (UserRoleRecord) TableName() string           { return "user_roles" }
func (APIKeyRecord) TableName() string             { return "api_keys" }
func (AdminSessionRecord) TableName() string       { return "admin_sessions" }
func (ProviderRecord) TableName() string           { return "providers" }
func (ProviderCredentialRecord) TableName() string { return "provider_credentials" }
func (ModelMappingRecord) TableName() string       { return "model_mappings" }
func (AdapterRecord) TableName() string            { return "adapters" }
func (AdapterInstanceRecord) TableName() string    { return "adapter_instances" }
func (AccountSourceRecord) TableName() string      { return "account_source_records" }
func (AccountRecord) TableName() string            { return "account_records" }
func (AccountLeaseRecord) TableName() string       { return "account_leases" }
func (UsageEventRecord) TableName() string         { return "usage_events" }
func (RequestLogRecord) TableName() string         { return "request_logs" }

func userToRecord(user domain.User) UserRecord {
	return UserRecord(user)
}

func userFromRecord(record UserRecord) domain.User {
	return domain.User(record)
}

func usersFromRecords(records []UserRecord) []domain.User {
	users := make([]domain.User, len(records))
	for i, record := range records {
		users[i] = userFromRecord(record)
	}
	return users
}

func roleToRecord(role domain.Role) RoleRecord {
	return RoleRecord(role)
}

func roleFromRecord(record RoleRecord) domain.Role {
	return domain.Role(record)
}

func rolesFromRecords(records []RoleRecord) []domain.Role {
	roles := make([]domain.Role, len(records))
	for i, record := range records {
		roles[i] = roleFromRecord(record)
	}
	return roles
}

func userRoleToRecord(userRole domain.UserRole) UserRoleRecord {
	return UserRoleRecord(userRole)
}

func userRoleFromRecord(record UserRoleRecord) domain.UserRole {
	return domain.UserRole(record)
}

func userRolesFromRecords(records []UserRoleRecord) []domain.UserRole {
	userRoles := make([]domain.UserRole, len(records))
	for i, record := range records {
		userRoles[i] = userRoleFromRecord(record)
	}
	return userRoles
}

func apiKeyToRecord(apiKey domain.APIKey) APIKeyRecord {
	return APIKeyRecord(apiKey)
}

func apiKeyFromRecord(record APIKeyRecord) domain.APIKey {
	return domain.APIKey(record)
}

func apiKeysFromRecords(records []APIKeyRecord) []domain.APIKey {
	apiKeys := make([]domain.APIKey, len(records))
	for i, record := range records {
		apiKeys[i] = apiKeyFromRecord(record)
	}
	return apiKeys
}

func adminSessionToRecord(session domain.AdminSession) AdminSessionRecord {
	return AdminSessionRecord(session)
}

func adminSessionFromRecord(record AdminSessionRecord) domain.AdminSession {
	return domain.AdminSession(record)
}

func providerToRecord(provider domain.Provider) ProviderRecord {
	return ProviderRecord(provider)
}

func providerFromRecord(record ProviderRecord) domain.Provider {
	return domain.Provider(record)
}

func providersFromRecords(records []ProviderRecord) []domain.Provider {
	providers := make([]domain.Provider, len(records))
	for i, record := range records {
		providers[i] = providerFromRecord(record)
	}
	return providers
}

func credentialToRecord(credential domain.ProviderCredential) ProviderCredentialRecord {
	return ProviderCredentialRecord(credential)
}

func mappingToRecord(mapping domain.ModelMapping) ModelMappingRecord {
	return ModelMappingRecord(mapping)
}

func mappingFromRecord(record ModelMappingRecord) domain.ModelMapping {
	return domain.ModelMapping(record)
}

func mappingsFromRecords(records []ModelMappingRecord) []domain.ModelMapping {
	mappings := make([]domain.ModelMapping, len(records))
	for i, record := range records {
		mappings[i] = mappingFromRecord(record)
	}
	return mappings
}

func adapterToRecord(adapter domain.Adapter) AdapterRecord {
	return AdapterRecord(adapter)
}

func adapterFromRecord(record AdapterRecord) domain.Adapter {
	return domain.Adapter(record)
}

func adaptersFromRecords(records []AdapterRecord) []domain.Adapter {
	adapters := make([]domain.Adapter, len(records))
	for i, record := range records {
		adapters[i] = adapterFromRecord(record)
	}
	return adapters
}

func adapterInstanceToRecord(instance domain.AdapterInstance) AdapterInstanceRecord {
	return AdapterInstanceRecord(instance)
}

func adapterInstanceFromRecord(record AdapterInstanceRecord) domain.AdapterInstance {
	return domain.AdapterInstance(record)
}

func adapterInstancesFromRecords(records []AdapterInstanceRecord) []domain.AdapterInstance {
	instances := make([]domain.AdapterInstance, len(records))
	for i, record := range records {
		instances[i] = adapterInstanceFromRecord(record)
	}
	return instances
}

func sourceToRecord(source domain.AccountSource) AccountSourceRecord {
	return AccountSourceRecord(source)
}

func sourceFromRecord(record AccountSourceRecord) domain.AccountSource {
	return domain.AccountSource(record)
}

func accountToRecord(account domain.Account) AccountRecord {
	return AccountRecord(account)
}

func accountsToRecords(accounts []domain.Account) []AccountRecord {
	records := make([]AccountRecord, len(accounts))
	for i, account := range accounts {
		records[i] = accountToRecord(account)
	}
	return records
}

func accountFromRecord(record AccountRecord) domain.Account {
	return domain.Account(record)
}

func accountsFromRecords(records []AccountRecord) []domain.Account {
	accounts := make([]domain.Account, len(records))
	for i, record := range records {
		accounts[i] = accountFromRecord(record)
	}
	return accounts
}

func leaseFromRecord(record AccountLeaseRecord) domain.AccountLease {
	return domain.AccountLease(record)
}

func usageToRecord(event domain.UsageEvent) UsageEventRecord {
	return UsageEventRecord(event)
}

func usageFromRecord(record UsageEventRecord) domain.UsageEvent {
	return domain.UsageEvent(record)
}

func usageEventsFromRecords(records []UsageEventRecord) []domain.UsageEvent {
	events := make([]domain.UsageEvent, len(records))
	for i, record := range records {
		events[i] = usageFromRecord(record)
	}
	return events
}

func requestLogToRecord(log domain.RequestLog) RequestLogRecord {
	return RequestLogRecord(log)
}

func requestLogFromRecord(record RequestLogRecord) domain.RequestLog {
	return domain.RequestLog(record)
}

func requestLogsFromRecords(records []RequestLogRecord) []domain.RequestLog {
	logs := make([]domain.RequestLog, len(records))
	for i, record := range records {
		logs[i] = requestLogFromRecord(record)
	}
	return logs
}
