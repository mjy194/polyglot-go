package telemetry

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"polyglot/pkg/logger"
)

const (
	ContextKeyRequestID = "telemetry_request_id"
	ContextKeyTraceID   = "telemetry_trace_id"
	ContextKeyRecorder  = "telemetry_recorder"

	HeaderRequestID = "X-Request-Id"
	HeaderTraceID   = "X-Trace-Id"
)

type Recorder struct {
	log       logger.Logger
	requestID string
	traceID   string
	start     time.Time
	fields    map[string]interface{}
}

type Span struct {
	recorder *Recorder
	name     string
	start    time.Time
	fields   []interface{}
}

func Middleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = "req_" + uuid.NewString()
		}
		traceID := c.GetHeader(HeaderTraceID)
		if traceID == "" {
			traceID = requestID
		}

		recorder := &Recorder{
			log:       log,
			requestID: requestID,
			traceID:   traceID,
			start:     time.Now(),
			fields: map[string]interface{}{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
			},
		}

		c.Set(ContextKeyRequestID, requestID)
		c.Set(ContextKeyTraceID, traceID)
		c.Set(ContextKeyRecorder, recorder)
		c.Writer.Header().Set(HeaderRequestID, requestID)
		c.Writer.Header().Set(HeaderTraceID, traceID)

		c.Next()

		path := routePath(c)
		latencyMs := millisSince(recorder.start)
		fields := []interface{}{
			"request_id", requestID,
			"trace_id", traceID,
			"event", "request.complete",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", latencyMs,
			"response_bytes", c.Writer.Size(),
			"client_ip", c.ClientIP(),
		}
		fields = append(fields, flattenMap(recorder.fields)...)
		ObserveRequest(c.Request.Method, path, c.Writer.Status(), latencyMs, recorder.fields)
		log.Info("telemetry", fields...)
	}
}

func FromGin(c *gin.Context) *Recorder {
	raw, ok := c.Get(ContextKeyRecorder)
	if !ok {
		return nil
	}
	recorder, _ := raw.(*Recorder)
	return recorder
}

func RequestID(c *gin.Context) string {
	if raw, ok := c.Get(ContextKeyRequestID); ok {
		if value, ok := raw.(string); ok {
			return value
		}
	}
	return ""
}

func TraceID(c *gin.Context) string {
	if raw, ok := c.Get(ContextKeyTraceID); ok {
		if value, ok := raw.(string); ok {
			return value
		}
	}
	return ""
}

func SetField(c *gin.Context, key string, value interface{}) {
	if recorder := FromGin(c); recorder != nil {
		recorder.fields[key] = value
	}
}

func Start(c *gin.Context, name string, fields ...interface{}) *Span {
	recorder := FromGin(c)
	return &Span{
		recorder: recorder,
		name:     name,
		start:    time.Now(),
		fields:   fields,
	}
}

func (s *Span) End(fields ...interface{}) {
	if s == nil || s.recorder == nil || s.recorder.log == nil {
		return
	}
	durationMs := millisSince(s.start)
	all := []interface{}{
		"request_id", s.recorder.requestID,
		"trace_id", s.recorder.traceID,
		"event", "span.complete",
		"span", s.name,
		"duration_ms", durationMs,
	}
	all = append(all, s.fields...)
	all = append(all, fields...)
	ObserveSpan(s.name, durationMs, all)
	s.recorder.log.Info("telemetry", all...)
}

func (s *Span) EndError(err error, fields ...interface{}) {
	if err == nil {
		s.End(fields...)
		return
	}
	fields = append(fields, "error", err.Error())
	s.End(fields...)
}

func Event(c *gin.Context, name string, fields ...interface{}) {
	recorder := FromGin(c)
	if recorder == nil || recorder.log == nil {
		return
	}
	all := []interface{}{
		"request_id", recorder.requestID,
		"trace_id", recorder.traceID,
		"event", name,
	}
	all = append(all, fields...)
	ObserveEvent(name, fields)
	recorder.log.Info("telemetry", all...)
}

func routePath(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return "unknown"
}

func millisSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func flattenMap(values map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(values)*2)
	for key, value := range values {
		if key == "" || value == nil {
			continue
		}
		out = append(out, "ctx_"+key, value)
	}
	return out
}

func Int64Field(value int64) string {
	return strconv.FormatInt(value, 10)
}

func Stringer(value interface{}) string {
	return fmt.Sprintf("%v", value)
}
