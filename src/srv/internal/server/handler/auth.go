package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polyglot/internal/authn"
	"polyglot/internal/data"
	"polyglot/internal/domain"
	"polyglot/internal/server/middleware"
)

const adminSessionTTL = 24 * time.Hour

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type bootstrapRequest struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type loginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      domain.User `json:"user"`
	Scopes    []string    `json:"scopes"`
}

func AuthBootstrap(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := store.Identity().CountPasswordUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "bootstrap already completed"})
			return
		}

		var req bootstrapRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := normalizedLogin(req.Email, req.Username)
		if email == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
			return
		}
		passwordHash, err := authn.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		user := domain.User{
			ID:           strings.TrimSpace(req.UserID),
			Email:        email,
			DisplayName:  strings.TrimSpace(req.DisplayName),
			PasswordHash: passwordHash,
			Status:       domain.StatusActive,
		}
		if user.DisplayName == "" {
			user.DisplayName = email
		}
		user, err = store.Identity().UpsertUser(c.Request.Context(), user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		role, err := store.Identity().UpsertRole(c.Request.Context(), domain.Role{
			Name:        "admin",
			Description: "Bootstrap administrator",
			Permissions: `["*"]`,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := store.Identity().AssignRole(c.Request.Context(), domain.UserRole{UserID: user.ID, RoleID: role.ID}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		writeLoginSession(c, store, user, []string{"admin"})
	}
}

func AuthLogout(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := authBearerToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing admin session"})
			return
		}
		if err := store.Identity().RevokeAdminSessionByTokenHash(c.Request.Context(), authn.TokenHash(token)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func AuthLogin(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		email := normalizedLogin(req.Email, req.Username)
		if email == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
			return
		}

		user, found, err := store.Identity().GetUserByEmail(c.Request.Context(), email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found || user.Status == domain.StatusDisabled || !authn.VerifyPassword(req.Password, user.PasswordHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		scopes, ok, err := adminScopes(c, store, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
			return
		}

		now := time.Now().UTC()
		user.LastLoginAt = &now
		user, err = store.Identity().UpsertUser(c.Request.Context(), user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		writeLoginSession(c, store, user, scopes)
	}
}

func AuthProfile(store *data.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(middleware.ContextUserID)
		id, _ := userID.(string)
		if id == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user"})
			return
		}
		user, found, err := store.Identity().GetUser(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !found || user.Status == domain.StatusDisabled {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func writeLoginSession(c *gin.Context, store *data.Store, user domain.User, scopes []string) {
	token, err := authn.NewSessionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	expiresAt := time.Now().UTC().Add(adminSessionTTL)
	rawScopes, err := json.Marshal(scopes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := store.Identity().CreateAdminSession(c.Request.Context(), domain.AdminSession{
		UserID:    user.ID,
		TokenHash: authn.TokenHash(token),
		Scopes:    string(rawScopes),
		Status:    domain.StatusActive,
		ExpiresAt: expiresAt,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user,
		Scopes:    scopes,
	})
}

func adminScopes(c *gin.Context, store *data.Store, userID string) ([]string, bool, error) {
	userRoles, err := store.Identity().ListUserRoles(c.Request.Context(), userID)
	if err != nil {
		return nil, false, err
	}
	for _, userRole := range userRoles {
		role, found, err := store.Identity().GetRole(c.Request.Context(), userRole.RoleID)
		if err != nil {
			return nil, false, err
		}
		if !found {
			continue
		}
		permissions := parsePermissionScopes(role.Permissions)
		if strings.EqualFold(role.Name, "admin") || permissionAllowed(permissions, "admin") {
			return []string{"admin"}, true, nil
		}
	}
	return nil, false, nil
}

func normalizedLogin(email, username string) string {
	value := strings.TrimSpace(email)
	if value == "" {
		value = strings.TrimSpace(username)
	}
	return strings.ToLower(value)
}

func authBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

func parsePermissionScopes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err == nil {
		return scopes
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		if scope := strings.TrimSpace(part); scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func permissionAllowed(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == required {
			return true
		}
	}
	return false
}
