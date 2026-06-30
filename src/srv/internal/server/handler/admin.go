package handler

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polyglot/internal/authn"
	"polyglot/internal/data"
	"polyglot/internal/domain"
	proxypkg "polyglot/internal/proxy"
	"polyglot/internal/server/middleware"
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
		// API keys are the user's own config — scope to the logged-in user.
		userID := c.GetString(middleware.ContextUserID)
		keys, err := store.Identity().ListAPIKeysByUser(c.Request.Context(), userID)
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
		// Force ownership: a key is always created under the logged-in user.
		key.UserID = c.GetString(middleware.ContextUserID)
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

// AdminDeleteAPIKey removes one of the current user's own API keys.
func AdminDeleteAPIKey(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Identity().DeleteAPIKey(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
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
	Status     string `json:"status"`
}

// AdminProviderHealth returns 24h per-provider request health (success rate +
// latency + total), keyed by provider name. Used for the Providers health bars.
func AdminProviderHealth(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		from := time.Now().Add(-24 * time.Hour)
		stats, err := store.Audit().RequestStatsByProvider(c.Request.Context(), from)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type health struct {
			RequestsTotal    int64   `json:"requests_total"`
			SuccessRate      float64 `json:"success_rate"`
			AverageLatencyMs float64 `json:"avg_latency_ms"`
		}
		out := make(map[string]health, len(stats))
		for name, s := range stats {
			out[name] = health{RequestsTotal: s.RequestsTotal, SuccessRate: s.SuccessRate, AverageLatencyMs: s.AverageLatencyMs}
		}
		c.JSON(http.StatusOK, out)
	}
}

// AdminProviderHealthHourly returns per-provider 24-hourly health buckets.
// Slot 0 = oldest (23h ago), slot 23 = current hour.
func AdminProviderHealthHourly(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		buckets, err := store.Audit().HourlyHealthByProvider(c.Request.Context(), now.Add(-24*time.Hour), now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, buckets)
	}
}

// AdminDeleteProvider removes a provider and its child associations.
func AdminDeleteProvider(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Providers().DeleteProvider(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
	}
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
				v.Name, v.URL, v.Status = p.Name, p.URL, p.Status
			}
			out = append(out, v)
		}
		c.JSON(http.StatusOK, out)
	}
}

// AdminTestProxy verifies a proxy by issuing a GET through it to a target URL.
// Body (optional): {"target": "https://..."} — defaults to a small 204 endpoint.
// Returns success + status code + latency, or an error message.
func AdminTestProxy(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Target string `json:"target"`
		}
		_ = c.ShouldBindJSON(&req)
		target := req.Target
		if target == "" {
			target = "https://www.gstatic.com/generate_204"
		}

		proxy, found, err := store.Proxies().GetProxy(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "proxy not found"})
			return
		}

		proxyURL := proxypkg.EmbedCredentials(proxy.URL, proxy.Username, proxy.Password)
		client := proxypkg.ClientFor(proxyURL, 12*time.Second)

		start := time.Now()
		resp, err := client.Get(target)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success":    false,
				"proxy":      proxy.Name,
				"target":     target,
				"latency_ms": latency,
				"error":      err.Error(),
				"exit_ip":    "",
			})
			return
		}
		defer resp.Body.Close()
		// If the target is an IP-echo endpoint, surface the exit IP.
		exitIP := ""
		if strings.Contains(target, "ipify.org") || strings.Contains(target, "ifconfig.me") || strings.Contains(target, "icanhazip.com") {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			exitIP = strings.TrimSpace(string(body))
		}
		c.JSON(http.StatusOK, gin.H{
			"success":    resp.StatusCode < 400,
			"proxy":      proxy.Name,
			"target":     target,
			"status":     resp.StatusCode,
			"latency_ms": latency,
			"exit_ip":    exitIP,
		})
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

// AdminListProviderModelMappings lists the model mappings owned by a provider.
func AdminListProviderModelMappings(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		mappings, err := store.Providers().ListModelMappingsByProvider(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, mappings)
	}
}

// AdminUpsertProviderModelMapping creates/updates a model mapping under a provider
// (provider_id is taken from the path, not the body).
func AdminUpsertProviderModelMapping(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var mapping domain.ModelMapping
		if err := c.ShouldBindJSON(&mapping); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		mapping.ProviderID = c.Param("id")
		mapping, err := store.Providers().UpsertModelMapping(c.Request.Context(), mapping)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, mapping)
	}
}

// AdminDeleteModelMapping deletes a model mapping by id.
func AdminDeleteModelMapping(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Providers().DeleteModelMapping(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
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
		// Enrich with user email + api key name for display.
		userName := map[string]string{}
		apiKeyName := map[string]string{}
		if users, err := store.Identity().ListUsers(c.Request.Context()); err == nil {
			for _, u := range users {
				userName[u.ID] = u.Email
			}
		}
		if keys, err := store.Identity().ListAPIKeys(c.Request.Context()); err == nil {
			for _, k := range keys {
				apiKeyName[k.ID] = k.Name
			}
		}
		type view struct {
			domain.RequestLog
			UserName   string `json:"user_name"`
			APIKeyName string `json:"api_key_name"`
		}
		out := make([]view, 0, len(logs))
		for _, l := range logs {
			out = append(out, view{RequestLog: l, UserName: userName[l.UserID], APIKeyName: apiKeyName[l.APIKeyID]})
		}
		c.JSON(http.StatusOK, out)
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

// ---- Groups (access/billing tier between users/keys and providers) ----

func AdminGroups(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := store.Groups().ListGroups(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, groups)
	}
}

func AdminUpsertGroup(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var group domain.Group
		if err := c.ShouldBindJSON(&group); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		group, err := store.Groups().UpsertGroup(c.Request.Context(), group)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, group)
	}
}

func AdminDeleteGroup(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.Groups().DeleteGroup(c.Request.Context(), c.Param("id")); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"deleted": c.Param("id")})
	}
}

// groupProviderView enriches a provider association with provider details.
type groupProviderView struct {
	GroupID    string `json:"group_id"`
	ProviderID string `json:"provider_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Priority   int    `json:"priority"`
}

func AdminListGroupProviders(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		links, err := store.Groups().ListGroupProviders(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		all, _ := store.Providers().ListProviders(c.Request.Context())
		byID := make(map[string]domain.Provider, len(all))
		for _, p := range all {
			byID[p.ID] = p
		}
		out := make([]groupProviderView, 0, len(links))
		for _, l := range links {
			v := groupProviderView{GroupID: l.GroupID, ProviderID: l.ProviderID, Priority: l.Priority}
			if p, ok := byID[l.ProviderID]; ok {
				v.Name, v.Type, v.Status = p.Name, p.Type, p.Status
			}
			out = append(out, v)
		}
		c.JSON(http.StatusOK, out)
	}
}

func AdminSetGroupProviders(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("id")
		var reqs []domain.GroupProvider
		if err := c.ShouldBindJSON(&reqs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Groups().SetGroupProviders(c.Request.Context(), groupID, reqs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"group_id": groupID, "count": len(reqs)})
	}
}

// providerGroupView enriches a group association with the group's name/ratio.
type providerGroupView struct {
	GroupID  string  `json:"group_id"`
	Name     string  `json:"name"`
	Ratio    float64 `json:"ratio"`
	Priority int     `json:"priority"`
}

func AdminListProviderGroups(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		links, err := store.Groups().ListProviderGroups(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		all, _ := store.Groups().ListGroups(c.Request.Context())
		byID := make(map[string]domain.Group, len(all))
		for _, g := range all {
			byID[g.ID] = g
		}
		out := make([]providerGroupView, 0, len(links))
		for _, l := range links {
			v := providerGroupView{GroupID: l.GroupID, Priority: l.Priority}
			if g, ok := byID[l.GroupID]; ok {
				v.Name, v.Ratio = g.Name, g.Ratio
			}
			out = append(out, v)
		}
		c.JSON(http.StatusOK, out)
	}
}

func AdminSetProviderGroups(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerID := c.Param("id")
		var reqs []domain.GroupProvider
		if err := c.ShouldBindJSON(&reqs); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Groups().SetProviderGroups(c.Request.Context(), providerID, reqs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"provider_id": providerID, "count": len(reqs)})
	}
}
