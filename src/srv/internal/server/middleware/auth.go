package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polyglot/internal/authn"
	"polyglot/internal/data"
	"polyglot/internal/domain"
	"polyglot/internal/telemetry"
)

const (
	ContextAPIKeyID      = "api_key_id"
	ContextUserID        = "user_id"
	ContextScopes        = "scopes"
	ContextAuditProvider = "audit_provider" // set by routing: the DB provider name that served the request
)

func APIKeyAuth(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "api key store unavailable"})
			return
		}
		key := apiKeyFromRequest(c)
		if key == "" {
			telemetry.Event(c, "auth.api_key", "result", "missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			return
		}
		span := telemetry.Start(c, "auth.api_key_lookup")
		record, found, err := store.Identity().GetAPIKeyByKey(c.Request.Context(), key)
		if err != nil {
			span.EndError(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found || !apiKeyActive(record, time.Now().UTC()) {
			span.End("result", "invalid")
			telemetry.Event(c, "auth.api_key", "result", "invalid")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		span.End("result", "ok", "api_key_id", record.ID)
		telemetry.Event(c, "auth.api_key", "result", "ok", "api_key_id", record.ID)
		c.Set(ContextAPIKeyID, record.ID)
		c.Set(ContextUserID, record.UserID)
		c.Set(ContextScopes, parseScopes(record.Scopes))
		c.Next()
	}
}

func AdminAuth(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin session store unavailable"})
			return
		}
		token := bearerTokenFromRequest(c)
		if token == "" {
			telemetry.Event(c, "auth.admin_session", "result", "missing")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing admin session"})
			return
		}
		span := telemetry.Start(c, "auth.admin_session_lookup")
		session, found, err := store.Identity().GetAdminSessionByTokenHash(c.Request.Context(), authn.TokenHash(token))
		if err != nil {
			span.EndError(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found || !adminSessionActive(session, time.Now().UTC()) {
			span.End("result", "invalid")
			telemetry.Event(c, "auth.admin_session", "result", "invalid")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin session"})
			return
		}
		span.End("result", "ok", "session_id", session.ID, "user_id", session.UserID)
		telemetry.Event(c, "auth.admin_session", "result", "ok", "session_id", session.ID)
		c.Set(ContextUserID, session.UserID)
		c.Set(ContextScopes, parseScopes(session.Scopes))
		c.Next()
	}
}

func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hasScope(scopesFromContext(c), scope) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
	}
}

func apiKeyFromRequest(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return strings.TrimSpace(c.GetHeader("x-api-key"))
}

func bearerTokenFromRequest(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

func apiKeyActive(key domain.APIKey, now time.Time) bool {
	if key.Status != "" && key.Status != domain.StatusActive {
		return false
	}
	return key.ExpiresAt == nil || key.ExpiresAt.After(now)
}

func adminSessionActive(session domain.AdminSession, now time.Time) bool {
	if session.Status != "" && session.Status != domain.StatusActive {
		return false
	}
	return session.ExpiresAt.After(now)
}

func parseScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err == nil {
		return scopes
	}
	parts := strings.Split(raw, ",")
	scopes = scopes[:0]
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

func scopesFromContext(c *gin.Context) []string {
	raw, ok := c.Get(ContextScopes)
	if !ok {
		return nil
	}
	scopes, _ := raw.([]string)
	return scopes
}

func hasScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == required {
			return true
		}
	}
	return false
}
