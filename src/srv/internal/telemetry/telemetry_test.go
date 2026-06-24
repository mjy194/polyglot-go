package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type testLogger struct {
	entries int
}

func (l *testLogger) Debug(string, ...interface{}) {}
func (l *testLogger) Info(string, ...interface{})  { l.entries++ }
func (l *testLogger) Warn(string, ...interface{})  {}
func (l *testLogger) Error(string, ...interface{}) {}
func (l *testLogger) Fatal(string, ...interface{}) {}
func (l *testLogger) Sync() error                  { return nil }

func TestMiddlewareSetsRequestAndTraceIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := &testLogger{}
	router := gin.New()
	router.Use(Middleware(log))
	router.GET("/x", func(c *gin.Context) {
		if RequestID(c) != "req_fixed" {
			t.Fatalf("request id = %q", RequestID(c))
		}
		SetField(c, "route_mode", "native")
		span := Start(c, "test.span")
		span.End("ok", true)
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", http.NoBody)
	req.Header.Set(HeaderRequestID, "req_fixed")
	router.ServeHTTP(w, req)

	if w.Header().Get(HeaderRequestID) != "req_fixed" {
		t.Fatalf("response request id = %q", w.Header().Get(HeaderRequestID))
	}
	if log.entries < 2 {
		t.Fatalf("expected span and request logs, got %d entries", log.entries)
	}
}

func TestMetricsRenderPrometheus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultMetrics.Reset()

	router := gin.New()
	router.Use(Middleware(&testLogger{}))
	router.GET("/x/:id", func(c *gin.Context) {
		SetField(c, "protocol", "openai")
		SetField(c, "route_mode", "universal")
		span := Start(c, "adapter.universal_stream", "protocol", "openai")
		span.End()
		Event(c, "adapter.first_event", "protocol", "openai", "ttfb_ms", int64(12))
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x/123", http.NoBody)
	router.ServeHTTP(w, req)

	out := defaultMetrics.RenderPrometheus()
	for _, want := range []string{
		`polyglot_http_requests_total{method="GET",path="/x/:id",status="204",protocol="openai",route_mode="universal"} 1`,
		`polyglot_http_request_latency_ms_count{method="GET",path="/x/:id",status="204",protocol="openai",route_mode="universal"} 1`,
		`polyglot_spans_total{span="adapter.universal_stream",protocol="openai",result="ok"} 1`,
		`polyglot_span_duration_ms_count{span="adapter.universal_stream",protocol="openai",result="ok"} 1`,
		`polyglot_adapter_ttfb_ms_count{event="adapter.first_event",protocol="openai"} 1`,
		`polyglot_events_total{event="adapter.first_event",protocol="openai",result="unknown"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics missing %q:\n%s", want, out)
		}
	}
}
