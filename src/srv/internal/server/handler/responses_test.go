package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polyglot/internal/adapter"
	pb "polyglot/proto/adapter"

	"github.com/gin-gonic/gin"
)

func newResponsesRouter(resolve StreamProcessorResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/r", Responses(resolve))
	return r
}

func doResponsesRequest(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/r", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// 非流式：input 为字符串
func TestResponsesNonStreamInputString(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hi there", 0, true),
		completionMsg("stop", 10, 20),
	}}
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return fp, true })

	w := doResponsesRequest(t, r, `{"model":"gpt-4o","input":"say hi"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["object"] != "response" {
		t.Errorf("object=%v, want response", resp["object"])
	}
	if resp["status"] != "completed" {
		t.Errorf("status=%v, want completed", resp["status"])
	}
	out := resp["output"].([]interface{})[0].(map[string]interface{})
	content := out["content"].([]interface{})[0].(map[string]interface{})
	if content["type"] != "output_text" || content["text"] != "Hi there" {
		t.Errorf("output_text=%v, want 'Hi there'", content)
	}
}

// 非流式：input 为 message items 数组（codex 实际发的形态）
func TestResponsesNonStreamInputItems(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hello", 0, true),
		completionMsg("stop", 1, 2),
	}}
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return fp, true })
	body := `{"model":"gpt-4o","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`
	w := doResponsesRequest(t, r, body)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	out := resp["output"].([]interface{})[0].(map[string]interface{})
	if out["type"] != "message" || out["role"] != "assistant" {
		t.Errorf("output item wrong: %v", out)
	}
}

// 流式：必须发出 Responses API 的事件序列
func TestResponsesStreamEvents(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hi", 0, false),
		chunkMsg(" there", 0, true),
		completionMsg("stop", 5, 8),
	}}
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return fp, true })
	body := `{"model":"gpt-4o","input":"hi","stream":true}`
	w := doResponsesRequest(t, r, body)
	s := w.Body.String()

	for _, want := range []string{
		"event: response.created",
		"event: response.output_item.added",
		"event: response.content_part.added",
		"event: response.output_text.delta",
		`"delta":"Hi"`,
		`"delta":" there"`,
		"event: response.output_text.done",
		"event: response.content_part.done",
		"event: response.output_item.done",
		"event: response.completed",
		`"total_tokens":13`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("Responses 流式缺少 %q\n--- 输出 ---\n%s", want, s)
		}
	}
}

func TestResponsesNoAdapterReturnsServiceUnavailable(t *testing.T) {
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return nil, false })
	w := doResponsesRequest(t, r, `{"model":"gpt-4o","input":"hi"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", w.Code)
	}
}

func TestResponsesStreamErrorDoesNotEmitCompletion(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		{Response: &pb.UniversalResponse_Error{Error: &pb.ErrorResponse{Message: "boom", Type: "server_error"}}},
	}}
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return fp, true })
	w := doResponsesRequest(t, r, `{"model":"gpt-4o","input":"hi","stream":true}`)
	s := w.Body.String()
	if !strings.Contains(s, `"message":"boom"`) {
		t.Fatalf("expected error event, got %s", s)
	}
	if strings.Contains(s, "response.completed") {
		t.Fatalf("should not emit completion after error: %s", s)
	}
}

func TestResponsesStreamFunctionCallEvents(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		toolCallMsg("call_1", "lookup", `{"city":"SF"}`, 0),
		completionMsg("tool_calls", 3, 5),
	}}
	r := newResponsesRouter(func() (adapter.StreamProcessor, bool) { return fp, true })
	w := doResponsesRequest(t, r, `{"model":"gpt-4o","input":"hi","stream":true}`)
	s := w.Body.String()

	for _, want := range []string{
		"event: response.output_item.added",
		"function_call",
		`"call_1"`,
		`"lookup"`,
		"event: response.function_call_arguments.delta",
		`"delta":"{\"city\":\"SF\"}"`,
		"event: response.output_item.done",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("Responses function call stream missing %q\n--- output ---\n%s", want, s)
		}
	}
}
