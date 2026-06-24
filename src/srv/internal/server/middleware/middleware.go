package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
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
		protocol := protocolFromPath(c.Request.URL.Path)
		// Wrap the writer to capture TTFT and tee a tail of the response to parse
		// usage (prompt/completion/cached tokens) for the audit log.
		cw := &captureWriter{ResponseWriter: c.Writer, maxTail: 64 * 1024}
		c.Writer = cw

		// Peek the request body to extract model + stream flag, then restore it.
		var model string
		var isStream bool
		if store != nil && protocol != "" && !shouldSkipAudit(c.FullPath()) && c.Request.Body != nil {
			if raw, err := io.ReadAll(c.Request.Body); err == nil {
				c.Request.Body.Close()
				c.Request.Body = io.NopCloser(bytes.NewReader(raw))
				var peek struct {
					Model  string `json:"model"`
					Stream bool   `json:"stream"`
				}
				_ = json.Unmarshal(raw, &peek)
				model = peek.Model
				isStream = peek.Stream
			}
		}

		c.Next()

		if store == nil || shouldSkipAudit(c.FullPath()) {
			telemetry.Event(c, "audit.skip", "reason", "store_or_path")
			return
		}
		if protocol == "" {
			telemetry.Event(c, "audit.skip", "reason", "unknown_protocol")
			return
		}
		span := telemetry.Start(c, "audit.persist", "protocol", protocol)
		// Prefer the provider resolved by routing (DB provider name); fall back to
		// the configured default when no DB provider handled the request.
		if resolved := c.GetString(ContextAuditProvider); resolved != "" {
			provider = resolved
		}
		var ttft int64
		if !cw.firstByte.IsZero() {
			ttft = cw.firstByte.Sub(start).Milliseconds()
		}
		in, out, cached := cw.usage()
		reqType := "nonstream"
		if isStream {
			reqType = "stream"
		}
		if err := store.Audit().RecordRequest(c.Request.Context(), domain.RequestLog{
			ID:            fmt.Sprintf("req_%d", start.UnixNano()),
			UserID:        stringContext(c, ContextUserID),
			APIKeyID:      stringContext(c, ContextAPIKeyID),
			Provider:      provider,
			Protocol:      protocol,
			Model:         model,
			StatusCode:    c.Writer.Status(),
			Success:       c.Writer.Status() >= 200 && c.Writer.Status() < 400,
			LatencyMs:     time.Since(start).Milliseconds(),
			ClientIP:      c.ClientIP(),
			Endpoint:      c.Request.URL.Path,
			TTFTMs:        ttft,
			InputTokens:   in,
			OutputTokens:  out,
			CachedTokens:  cached,
			Type:          reqType,
			CreatedAt:     start.UTC(),
		}); err != nil {
			span.EndError(err)
			c.Error(err)
			return
		}
		span.End("status", c.Writer.Status(), "success", c.Writer.Status() >= 200 && c.Writer.Status() < 400)
	}
}

// captureWriter records time-to-first-byte (≈ TTFT) and keeps a tail of the
// response to parse the usage object (prompt/completion/cached tokens) for the
// audit log. Usage appears at the end (non-stream JSON or the final SSE chunk),
// so a bounded tail is sufficient.
type captureWriter struct {
	gin.ResponseWriter
	firstByte time.Time
	tail      []byte
	maxTail   int
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.firstByte.IsZero() {
		w.firstByte = time.Now()
	}
	w.tail = append(w.tail, b...)
	if len(w.tail) > w.maxTail {
		w.tail = w.tail[len(w.tail)-w.maxTail:]
	}
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// usage scans the response tail for the last occurrence of prompt/completion/
// cached token counts. Returns zeros if not found.
func (w *captureWriter) usage() (prompt, completion, cached int64) {
	lastInt := func(pattern string) int64 {
		re := regexp.MustCompile(pattern)
		ms := re.FindAllStringSubmatch(string(w.tail), -1)
		if len(ms) == 0 {
			return 0
		}
		var v int64
		_, _ = fmt.Sscanf(ms[len(ms)-1][1], "%d", &v)
		return v
	}
	prompt = lastInt(`"prompt_tokens"\s*:\s*(\d+)`)
	completion = lastInt(`"completion_tokens"\s*:\s*(\d+)`)
	cached = lastInt(`"cached_tokens"\s*:\s*(\d+)`)
	return
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
