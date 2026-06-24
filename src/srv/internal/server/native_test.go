package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"polyglot/internal/adapter"
	pb "polyglot/proto/adapter"
)

type fakeNativeProcessor struct {
	seen *pb.NativeRequest
	msgs []*pb.NativeResponse
	err  error
}

func (f *fakeNativeProcessor) ProcessNative(ctx context.Context, req *pb.NativeRequest, onResp func(*pb.NativeResponse) error) error {
	f.seen = req
	for _, msg := range f.msgs {
		if err := onResp(msg); err != nil {
			return err
		}
	}
	return f.err
}

func TestServeNativeForwardsRawRequestAndResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	processor := &fakeNativeProcessor{msgs: []*pb.NativeResponse{{
		StatusCode: http.StatusAccepted,
		Headers:    map[string]string{"Content-Type": "application/x-ndjson"},
		Body:       []byte("{\"ok\":true}\n"),
		EndStream:  true,
	}}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", strings.NewReader(`{"stream":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "text/event-stream")

	if err := serveNative(c, processor, "responses", "responses", ""); err != nil {
		t.Fatalf("serveNative: %v", err)
	}
	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("Content-Type=%q", got)
	}
	if got := w.Body.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("body=%q", got)
	}
	if processor.seen.GetProtocol() != "responses" || processor.seen.GetEndpoint() != "responses" {
		t.Fatalf("native request route mismatch: %+v", processor.seen)
	}
	if !processor.seen.GetStream() {
		t.Fatalf("native request stream flag not detected")
	}
	if string(processor.seen.GetBody()) != `{"stream":true}` {
		t.Fatalf("body not forwarded: %q", string(processor.seen.GetBody()))
	}
}

func TestServeNativeNoResponseReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	processor := &fakeNativeProcessor{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"m"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	if err := serveNative(c, processor, "anthropic", "messages", ""); err != nil {
		t.Fatalf("serveNative: %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestServeNativeAdapterErrorBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	processor := &fakeNativeProcessor{err: errors.New("native failed")}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"m"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	err := serveNative(c, processor, "responses", "responses", "")
	if err == nil || !strings.Contains(err.Error(), "native failed") {
		t.Fatalf("error=%v, want native failed", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status should remain unwritten default before wrapper handles error, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("body should be empty before wrapper handles error, got %s", w.Body.String())
	}
}

func TestServeNativeResponseErrorBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	processor := &fakeNativeProcessor{msgs: []*pb.NativeResponse{{
		Error: &pb.ErrorResponse{Message: "upstream rejected", Type: "bad_request"},
	}}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"m"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	err := serveNative(c, processor, "responses", "responses", "")
	if err == nil || !strings.Contains(err.Error(), "upstream rejected") {
		t.Fatalf("error=%v, want upstream rejected", err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status should remain unwritten default before wrapper handles error, got %d", w.Code)
	}
}

func TestNativeRequestStreamDetectionMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		protocol string
		path     string
		accept   string
		body     string
		want     bool
	}{
		{name: "body stream true", protocol: "openai", path: "/v1/chat/completions", body: `{"stream":true}`, want: true},
		{name: "body stream false", protocol: "openai", path: "/v1/chat/completions", body: `{"stream":false}`, want: false},
		{name: "accept event stream", protocol: "responses", path: "/v1/responses", accept: "text/event-stream", body: `{}`, want: true},
		{name: "gemini alt sse", protocol: "gemini", path: "/v1beta/models/gemini-pro?alt=sse", body: `{}`, want: true},
		{name: "invalid json not stream", protocol: "openai", path: "/v1/chat/completions", body: `{`, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			if tc.accept != "" {
				c.Request.Header.Set("Accept", tc.accept)
			}
			if got := nativeRequestIsStream(tc.protocol, c, []byte(tc.body)); got != tc.want {
				t.Fatalf("nativeRequestIsStream=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestAdapterSupportsNativeMatchesEndpoint(t *testing.T) {
	metadata := &pb.AdapterMetadata{NativeProtocols: []*pb.NativeProtocolSupport{
		{Protocol: "openai", Endpoints: []string{"chat_completions"}},
		{Protocol: "gemini", Endpoints: []string{"*"}},
	}}

	if !adapter.SupportsNative(metadata, "openai", "chat_completions") {
		t.Fatalf("expected openai chat_completions support")
	}
	if adapter.SupportsNative(metadata, "openai", "responses") {
		t.Fatalf("unexpected openai responses support")
	}
	if !adapter.SupportsNative(metadata, "gemini", "generate_content") {
		t.Fatalf("expected wildcard gemini support")
	}
}
