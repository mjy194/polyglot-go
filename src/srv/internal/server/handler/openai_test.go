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

func newOpenAIRouter(resolve StreamProcessorResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/c", OpenAIChatCompletions(resolve))
	return r
}

func doOpenAIRequest(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/c", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// 非流式：连续文本 chunk 合并进单个 content，finish_reason=stop，model 回显。
func TestOpenAINonStreamText(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hello world", 0, true),
		completionMsg("stop", 10, 20),
	}}
	r := newOpenAIRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })

	w := doOpenAIRequest(t, r, `{"model":"gpt-4","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp["model"] != "gpt-4" {
		t.Errorf("model=%v, want gpt-4", resp["model"])
	}
	choices, _ := resp["choices"].([]interface{})
	if len(choices) != 1 {
		t.Fatalf("choices=%d, want 1", len(choices))
	}
	ch := choices[0].(map[string]interface{})
	if ch["finish_reason"] != "stop" {
		t.Errorf("finish_reason=%v, want stop", ch["finish_reason"])
	}
	msg := ch["message"].(map[string]interface{})
	if msg["content"] != "Hello world" {
		t.Errorf("content=%v, want 'Hello world'", msg["content"])
	}
}

func TestOpenAINonStreamFinishReasonMapped(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("x", 0, true),
		completionMsg("max_tokens", 1, 2),
	}}
	r := newOpenAIRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })
	w := doOpenAIRequest(t, r, `{"model":"gpt-4","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	ch := resp["choices"].([]interface{})[0].(map[string]interface{})
	if ch["finish_reason"] != "length" {
		t.Fatalf("finish_reason=%v, want length (max_tokens→length)", ch["finish_reason"])
	}
}

func TestOpenAIStreamTextSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hi", 0, false),
		chunkMsg(" there", 0, true),
		completionMsg("stop", 5, 8),
	}}
	r := newOpenAIRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })

	w := doOpenAIRequest(t, r, `{"model":"gpt-4","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	s := w.Body.String()

	for _, want := range []string{
		"data: ",
		`"role":"assistant"`,
		`"Hi"`,
		`" there"`,
		`"finish_reason":"stop"`,
		"data: [DONE]",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("OpenAI SSE 缺少 %q\n--- 输出 ---\n%s", want, s)
		}
	}
}

func TestOpenAINoAdapterReturnsServiceUnavailable(t *testing.T) {
	r := newOpenAIRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return nil, false })
	w := doOpenAIRequest(t, r, `{"model":"gpt-4","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 (无 adapter 时不应返回 mock)", w.Code)
	}
}

func TestOpenAIStreamToolCallSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		toolCallMsg("call_1", "lookup", `{"city":"SF"}`, 0),
		completionMsg("tool_calls", 3, 5),
	}}
	r := newOpenAIRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })

	w := doOpenAIRequest(t, r, `{"model":"gpt-4","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	s := w.Body.String()
	for _, want := range []string{
		`"tool_calls":[{"index":0,"id":"call_1","type":"function"`,
		`"name":"lookup"`,
		`"arguments":""`,
		`"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"SF\"}"}}]`,
		`"finish_reason":"tool_calls"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("OpenAI tool stream missing %q\n--- output ---\n%s", want, s)
		}
	}
}
