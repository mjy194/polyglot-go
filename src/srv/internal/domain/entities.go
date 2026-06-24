package domain

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusHealthy  = "healthy"

	LeaseStatusActive   = "active"
	LeaseStatusReleased = "released"
)

type User struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	PasswordHash string     `json:"-"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions string    `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserRole struct {
	UserID string `json:"user_id"`
	RoleID string `json:"role_id"`
}

type APIKey struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Key        string     `json:"key"`
	Scopes     string     `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AdminSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	Scopes    string    `json:"scopes"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Provider struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	BaseURL        string    `json:"base_url"`
	AuthType       string    `json:"auth_type"`
	DefaultHeaders string    `json:"default_headers"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProviderCredential struct {
	ID         string
	ProviderID string
	Name       string
	SecretRef  string
	Status     string
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ModelMapping struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"provider_id"`
	FromModel  string    `json:"from_model"`
	ToModel    string    `json:"to_model"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Adapter struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AdapterInstance struct {
	ID              string
	AdapterID       string
	Provider        string
	CallbackAddr    string
	Capabilities    string
	Metadata        string
	Status          string
	LastHeartbeatAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AccountSource struct {
	SourceID     string
	Provider     string
	CallbackAddr string
	Capabilities string
	Watermark    string
	Status       string
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Account struct {
	AccountID   string
	SourceID    string
	Provider    string
	Credentials string
	Metadata    string
	ExpiresAt   time.Time
	Health      string
	InUse       bool
	UsageCount  int64
	LastUsedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AccountLease struct {
	ID         string
	AccountID  string
	SessionID  string
	RequestID  string
	Status     string
	AcquiredAt time.Time
	ReleasedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UsageEvent struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	AccountID     string    `json:"account_id"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	TokensUsed    int64     `json:"tokens_used"`
	RequestsCount int64     `json:"requests_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type RequestLog struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	APIKeyID     string    `json:"api_key_id"`
	Provider     string    `json:"provider"`
	AdapterID    string    `json:"adapter_id"`
	Protocol     string    `json:"protocol"`
	Model        string    `json:"model"`
	StatusCode   int       `json:"status_code"`
	Success      bool      `json:"success"`
	LatencyMs    int64     `json:"latency_ms"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
}
