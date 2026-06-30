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

func newGeminiRouter(resolve StreamProcessorResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/g/:model", GeminiGenerateContent(resolve))
	return r
}

func doGeminiRequest(t *testing.T, r *gin.Engine, body, query string) *httptest.ResponseRecorder {
	t.Helper()
	path := "/g/gemini-pro"
	if query != "" {
		path += "?" + query
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

const geminiBody = `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`

// 非流式：连续文本 chunk 合并进单个 part，finishReason=STOP，usage 正确。
func TestGeminiNonStreamText(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hi there", 0, true),
		completionMsg("stop", 10, 20),
	}}
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })

	w := doGeminiRequest(t, r, geminiBody, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	cands, _ := resp["candidates"].([]interface{})
	ch := cands[0].(map[string]interface{})
	if ch["finishReason"] != "STOP" {
		t.Errorf("finishReason=%v, want STOP", ch["finishReason"])
	}
	parts := ch["content"].(map[string]interface{})["parts"].([]interface{})
	if len(parts) != 1 {
		t.Fatalf("parts=%d, want 1 (merged text)", len(parts))
	}
	if parts[0].(map[string]interface{})["text"] != "Hi there" {
		t.Errorf("text=%v, want 'Hi there'", parts[0])
	}
}

// 关键回归：两个文本 chunk 必须合并成 1 个 part（修 UniversalToGemini 的 per-chunk part bug）。
func TestGeminiNonStreamMergesChunks(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hello", 0, false),
		chunkMsg(" world", 0, true),
		completionMsg("stop", 1, 2),
	}}
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })
	w := doGeminiRequest(t, r, geminiBody, "")
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	parts := resp["candidates"].([]interface{})[0].(map[string]interface{})["content"].(map[string]interface{})["parts"].([]interface{})
	if len(parts) != 1 || parts[0].(map[string]interface{})["text"] != "Hello world" {
		t.Fatalf("expected 1 merged part 'Hello world', got %v", parts)
	}
}

func TestGeminiStreamTextSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hi", 0, false),
		chunkMsg(" there", 0, true),
		completionMsg("max_tokens", 5, 8),
	}}
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })

	w := doGeminiRequest(t, r, geminiBody, "alt=sse")
	s := w.Body.String()
	for _, want := range []string{"data: ", `"Hi"`, `" there"`, "MAX_TOKENS", `"totalTokenCount":13`} {
		if !strings.Contains(s, want) {
			t.Errorf("Gemini SSE 缺少 %q\n--- 输出 ---\n%s", want, s)
		}
	}
}

func TestGeminiNoAdapterReturnsServiceUnavailable(t *testing.T) {
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return nil, false })
	w := doGeminiRequest(t, r, geminiBody, "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 (无 adapter 时不应返回 mock)", w.Code)
	}
}

func TestGeminiStreamErrorDoesNotEmitFinalStop(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		{Response: &pb.UniversalResponse_Error{Error: &pb.ErrorResponse{Message: "boom", Type: "server_error"}}},
	}}
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })
	w := doGeminiRequest(t, r, geminiBody, "alt=sse")
	s := w.Body.String()
	if !strings.Contains(s, `"message":"boom"`) {
		t.Fatalf("expected error payload, got %s", s)
	}
	if strings.Contains(s, `"finishReason":"STOP"`) {
		t.Fatalf("should not emit final STOP after error: %s", s)
	}
}

func TestGeminiStreamFunctionCallSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		toolCallMsg("call_1", "lookup", `{"city":"SF"}`, 0),
		completionMsg("tool_calls", 3, 5),
	}}
	r := newGeminiRouter(func(c *gin.Context) (adapter.StreamProcessor, bool) { return fp, true })
	w := doGeminiRequest(t, r, geminiBody, "alt=sse")
	s := w.Body.String()

	for _, want := range []string{
		`"functionCall":{"name":"lookup","args":{"city":"SF"}}`,
		`"finishReason":"STOP"`,
		`"totalTokenCount":8`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("Gemini function call stream missing %q\n--- output ---\n%s", want, s)
		}
	}
}
