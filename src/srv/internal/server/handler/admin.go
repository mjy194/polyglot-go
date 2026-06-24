package handler

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polyglot/internal/authn"
	"polyglot/internal/data"
	"polyglot/internal/domain"
)

// AdminStats returns request statistics from persisted request logs.
func AdminStats(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusOK, gin.H{
				"requests_total":     0,
				"success_rate":       0,
				"average_latency_ms": 0,
			})
			return
		}

		stats, err := store.Audit().RequestStats(c.Request.Context(), requestLogFilter(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"requests_total":     stats.RequestsTotal,
			"success_rate":       stats.SuccessRate,
			"average_latency_ms": stats.AverageLatencyMs,
		})
	}
}

func AdminProviders(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		providers, err := store.Providers().ListProviders(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, providers)
	}
}

func AdminUsers(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := store.Identity().ListUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

func AdminUpsertUser(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req adminUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		user, err := req.domain()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if user.Status == "" {
			user.Status = domain.StatusActive
		}
		user, err = store.Identity().UpsertUser(c.Request.Context(), user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

type adminUserRequest struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Password    string     `json:"password"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func (r adminUserRequest) domain() (domain.User, error) {
	email := normalizedLogin(r.Email, r.Username)
	user := domain.User{
		ID:          strings.TrimSpace(r.ID),
		Email:       email,
		DisplayName: strings.TrimSpace(r.DisplayName),
		Status:      strings.TrimSpace(r.Status),
		LastLoginAt: r.LastLoginAt,
	}
	if r.Password != "" {
		passwordHash, err := authn.HashPassword(r.Password)
		if err != nil {
			return domain.User{}, err
		}
		user.PasswordHash = passwordHash
	}
	return user, nil
}

func AdminRoles(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, err := store.Identity().ListRoles(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, roles)
	}
}

func AdminUpsertRole(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var role domain.Role
		if err := c.ShouldBindJSON(&role); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		role, err := store.Identity().UpsertRole(c.Request.Context(), role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, role)
	}
}

func AdminAssignRole(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userRole domain.UserRole
		if err := c.ShouldBindJSON(&userRole); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Identity().AssignRole(c.Request.Context(), userRole); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, userRole)
	}
}

func AdminUserRoles(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, err := store.Identity().ListUserRoles(c.Request.Context(), c.Query("user_id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, roles)
	}
}

func AdminAPIKeys(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		keys, err := store.Identity().ListAPIKeys(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out := make([]apiKeyResponse, len(keys))
		for i, key := range keys {
			out[i] = apiKeyResponseFromDomain(key)
		}
		c.JSON(http.StatusOK, out)
	}
}

func AdminUpsertAPIKey(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req apiKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		key, err := req.domain()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if key.Status == "" {
			key.Status = domain.StatusActive
		}
		key, err = store.Identity().UpsertAPIKey(c.Request.Context(), key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, apiKeyResponseFromDomain(key))
	}
}

type apiKeyRequest struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Key       string     `json:"key"`
	Scopes    string     `json:"scopes"`
	Status    string     `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type apiKeyResponse struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id,omitempty"`
	Name       string     `json:"name"`
	Key        string     `json:"key,omitempty"`
	Scopes     string     `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (r apiKeyRequest) domain() (domain.APIKey, error) {
	rawKey := r.Key
	if rawKey == "" {
		var err error
		rawKey, err = generateAPIKey()
		if err != nil {
			return domain.APIKey{}, err
		}
	}
	return domain.APIKey{
		ID:        r.ID,
		UserID:    r.UserID,
		Name:      r.Name,
		Key:       rawKey,
		Scopes:    r.Scopes,
		Status:    r.Status,
		ExpiresAt: r.ExpiresAt,
	}, nil
}

func apiKeyResponseFromDomain(key domain.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID:         key.ID,
		UserID:     key.UserID,
		Name:       key.Name,
		Key:        key.Key,
		Scopes:     key.Scopes,
		Status:     key.Status,
		ExpiresAt:  key.ExpiresAt,
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
		UpdatedAt:  key.UpdatedAt,
	}
}

func generateAPIKey() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "pk_" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func AdminUpsertProvider(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var provider domain.Provider
		if err := c.ShouldBindJSON(&provider); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if provider.Status == "" {
			provider.Status = domain.StatusActive
		}
		if provider.ProxyStrategy == "" {
			provider.ProxyStrategy = "failover"
		}
		provider, err := store.Providers().UpsertProvider(c.Request.Context(), provider)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, provider)
	}
}

// AdminProxies lists all network proxies.
func AdminProxies(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxies, err := store.Proxies().ListProxies(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, proxies)
	}
}

// AdminUpsertProxy creates or updates a network proxy.
func AdminUpsertProxy(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var proxy domain.Proxy
		if err := c.ShouldBindJSON(&proxy); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if proxy.Status == "" {
			proxy.Status = domain.StatusActive
		}
		proxy, err := store.Proxies().UpsertProxy(c.Request.Context(), proxy)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, proxy)
	}
}

// AdminDeleteProxy removes a network proxy (associations are left dangling;
// callers should clear provider↔proxy links first).
func AdminDeleteProxy(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Proxies().DeleteProxy(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
	}
}

// providerProxyView enriches an association with proxy details for the UI.
type providerProxyView struct {
	ProviderID string `json:"provider_id"`
	ProxyID    string `json:"proxy_id"`
	Priority   int    `json:"priority"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	Status     string `json:"status"`
}

// AdminListProviderProxies returns the proxies attached to a provider, enriched.
func AdminListProviderProxies(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerID := c.Param("id")
		links, err := store.Proxies().ListProviderProxies(c.Request.Context(), providerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		byID := make(map[string]domain.Proxy, len(links))
		if all, err := store.Proxies().ListProxies(c.Request.Context()); err == nil {
			for _, p := range all {
				byID[p.ID] = p
			}
		}
		out := make([]providerProxyView, 0, len(links))
		for _, l := range links {
			v := providerProxyView{ProviderID: l.ProviderID, ProxyID: l.ProxyID, Priority: l.Priority}
			if p, ok := byID[l.ProxyID]; ok {
				v.Name, v.URL, v.Type, v.Status = p.Name, p.URL, p.Type, p.Status
			}
			out = append(out, v)
		}
		c.JSON(http.StatusOK, out)
	}
}

// AdminSetProviderProxies replaces all provider↔proxy associations for a provider.
// Body: [{"proxy_id":"...","priority":0}, ...]
func AdminSetProviderProxies(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerID := c.Param("id")
		var reqs []domain.ProviderProxy
		if err := c.ShouldBindJSON(&reqs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Proxies().SetProviderProxies(c.Request.Context(), providerID, reqs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"provider_id": providerID, "count": len(reqs)})
	}
}

func writeFound(c *gin.Context, value interface{}, found bool, err error) {
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, value)
}

func AdminModelMappings(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		mappings, err := store.Providers().ListModelMappings(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, mappings)
	}
}

func AdminUpsertModelMapping(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var mapping domain.ModelMapping
		if err := c.ShouldBindJSON(&mapping); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		mapping, err := store.Providers().UpsertModelMapping(c.Request.Context(), mapping)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, mapping)
	}
}

func AdminAdapters(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		adapters, err := store.Adapters().ListAdapters(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, adapters)
	}
}

func AdminAdapterInstances(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		instances, err := store.Adapters().ListInstances(c.Request.Context(), c.Query("adapter_id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, instances)
	}
}

func AdminRequestLogs(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		logs, err := store.Audit().ListRequestLogs(c.Request.Context(), requestLogFilter(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, logs)
	}
}

func AdminUsageEvents(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		events, err := store.Audit().ListUsageEvents(c.Request.Context(), usageEventFilter(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, events)
	}
}

func requestLogFilter(c *gin.Context) data.RequestLogFilter {
	return data.RequestLogFilter{
		UserID:   c.Query("user_id"),
		Provider: c.Query("provider"),
		Protocol: c.Query("protocol"),
		From:     parseTimeQuery(c.Query("from")),
		To:       parseTimeQuery(c.Query("to")),
		Limit:    parseLimit(c.Query("limit")),
	}
}

func usageEventFilter(c *gin.Context) data.UsageEventFilter {
	return data.UsageEventFilter{
		UserID:    c.Query("user_id"),
		AccountID: c.Query("account_id"),
		Provider:  c.Query("provider"),
		Model:     c.Query("model"),
		From:      parseTimeQuery(c.Query("from")),
		To:        parseTimeQuery(c.Query("to")),
		Limit:     parseLimit(c.Query("limit")),
	}
}

func parseLimit(raw string) int {
	if raw == "" {
		return 100
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 100
	}
	if n > 1000 {
		return 1000
	}
	return n
}

func parseTimeQuery(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}
