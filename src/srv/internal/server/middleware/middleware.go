package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polyglot/internal/data"
	"polyglot/internal/domain"
	"polyglot/internal/telemetry"
	"polyglot/pkg/logger"
)

// Logger 日志中间件
func Logger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 处理请求
		c.Next()

		// 记录日志
		duration := time.Since(start)
		log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", duration.String(),
			"ip", c.ClientIP(),
		)
	}
}

// CORS CORS 中间件
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(origins) > 0 {
			origin := origins[0]
			if origin == "*" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			}
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func RequestAudit(store *data.Store, provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if store == nil || shouldSkipAudit(c.FullPath()) {
			telemetry.Event(c, "audit.skip", "reason", "store_or_path")
			return
		}
		protocol := protocolFromPath(c.Request.URL.Path)
		if protocol == "" {
			telemetry.Event(c, "audit.skip", "reason", "unknown_protocol")
			return
		}
		span := telemetry.Start(c, "audit.persist", "protocol", protocol)
		if err := store.Audit().RecordRequest(c.Request.Context(), domain.RequestLog{
			ID:         fmt.Sprintf("req_%d", start.UnixNano()),
			UserID:     stringContext(c, ContextUserID),
			APIKeyID:   stringContext(c, ContextAPIKeyID),
			Provider:   provider,
			Protocol:   protocol,
			StatusCode: c.Writer.Status(),
			Success:    c.Writer.Status() >= 200 && c.Writer.Status() < 400,
			LatencyMs:  time.Since(start).Milliseconds(),
			CreatedAt:  start.UTC(),
		}); err != nil {
			span.EndError(err)
			c.Error(err)
			return
		}
		span.End("status", c.Writer.Status(), "success", c.Writer.Status() >= 200 && c.Writer.Status() < 400)
	}
}

func stringContext(c *gin.Context, key string) string {
	raw, ok := c.Get(key)
	if !ok {
		return ""
	}
	value, _ := raw.(string)
	return value
}

func shouldSkipAudit(fullPath string) bool {
	return fullPath == "" || fullPath == "/health" || strings.HasPrefix(fullPath, "/api/admin")
}

func protocolFromPath(path string) string {
	switch {
	case path == "/v1/messages" || path == "/api/v1/messages":
		return "anthropic"
	case path == "/v1/chat/completions" || path == "/api/v1/chat/completions":
		return "openai"
	case path == "/v1/responses" || path == "/api/v1/responses":
		return "responses"
	case strings.HasPrefix(path, "/v1beta"):
		return "gemini"
	case strings.HasPrefix(path, "/api/v1beta/"):
		return "gemini"
	default:
		return ""
	}
}
