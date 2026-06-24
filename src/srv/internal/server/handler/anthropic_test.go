package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"polyglot/internal/adapter"

	pb "polyglot/proto/adapter"

	"github.com/gin-gonic/gin"
)

// fakeProcessor 实现 adapter.StreamProcessor，按预设序列回吐 UniversalResponse。
type fakeProcessor struct {
	msgs []*pb.UniversalResponse
	err  error
}

func (f fakeProcessor) ProcessStream(ctx context.Context, req *pb.UniversalRequest, onResp func(*pb.UniversalResponse) error) error {
	for _, m := range f.msgs {
		if err := onResp(m); err != nil {
			return err
		}
	}
	return f.err
}

func chunkMsg(text string, index int32, final bool) *pb.UniversalResponse {
	return &pb.UniversalResponse{Response: &pb.UniversalResponse_Chunk{Chunk: &pb.ContentChunk{Text: text, Index: index, IsFinal: final}}}
}

func completionMsg(reason string, in, out uint32) *pb.UniversalResponse {
	return &pb.UniversalResponse{Response: &pb.UniversalResponse_Completion{Completion: &pb.CompletionInfo{FinishReason: reason, InputTokens: in, OutputTokens: out}}}
}

func toolCallMsg(id, name, args string, index int32) *pb.UniversalResponse {
	return &pb.UniversalResponse{Response: &pb.UniversalResponse_ToolCall{ToolCall: &pb.ToolCall{Id: id, Name: name, Arguments: args, Index: index}}}
}

func newTestRouter(resolve StreamProcessorResolver) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/m", AnthropicMessages(resolve))
	return r
}

func doRequest(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/m", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestNonStreamTextResponse(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hello world", 0, true),
		completionMsg("stop", 10, 20),
	}}
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return fp, true })

	w := doRequest(t, r, `{"model":"claude-opus-4-8","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp["model"] != "claude-opus-4-8" {
		t.Errorf("model=%v, want claude-opus-4-8 (应取自请求而非 converter 的写死值)", resp["model"])
	}
	if resp["stop_reason"] != "end_turn" {
		t.Errorf("stop_reason=%v, want end_turn", resp["stop_reason"])
	}
	content, _ := resp["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("content blocks=%d, want 1", len(content))
	}
	blk, _ := content[0].(map[string]interface{})
	if blk["type"] != "text" || blk["text"] != "Hello world" {
		t.Errorf("content block = %v, want text 'Hello world'", blk)
	}
}

func TestNonStreamStopReasonMapped(t *testing.T) {
	// 上游 finish_reason=length → Anthropic max_tokens
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("x", 0, true),
		completionMsg("length", 1, 2),
	}}
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return fp, true })
	w := doRequest(t, r, `{"model":"m","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["stop_reason"] != "max_tokens" {
		t.Fatalf("stop_reason=%v, want max_tokens", resp["stop_reason"])
	}
}

func TestStreamTextSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		chunkMsg("Hello", 0, false),
		chunkMsg(" world", 0, true),
		completionMsg("stop", 10, 20),
	}}
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return fp, true })

	w := doRequest(t, r, `{"model":"claude-opus-4-8","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	s := w.Body.String()

	for _, want := range []string{
		"event: message_start",
		`"content":[]`,
		`"stop_reason":null`,
		"event: ping",
		"event: content_block_start",
		"event: content_block_delta",
		`"text_delta"`,
		`"Hello"`,
		`" world"`,
		"event: content_block_stop",
		"event: message_delta",
		`"end_turn"`,
		`"input_tokens":10`,
		`"output_tokens":20`,
		"event: message_stop",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SSE 缺少 %q\n--- 完整输出 ---\n%s", want, s)
		}
	}
	if strings.Contains(s, "event: input_json_delta") {
		t.Errorf("不应出现独立的 input_json_delta 事件\n%s", s)
	}
}

func TestStreamToolUseSSE(t *testing.T) {
	fp := fakeProcessor{msgs: []*pb.UniversalResponse{
		toolCallMsg("toolu_1", "get_weather", `{"location":"SF"}`, 0),
		completionMsg("tool_calls", 5, 8),
	}}
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return fp, true })

	w := doRequest(t, r, `{"model":"claude-opus-4-8","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"weather?"}]}`)
	s := w.Body.String()

	for _, want := range []string{
		`"type":"tool_use"`,
		`"name":"get_weather"`,
		"event: content_block_delta",
		`"input_json_delta"`,
		`"partial_json"`,
		`"tool_use"`, // message_delta 的 stop_reason
	} {
		if !strings.Contains(s, want) {
			t.Errorf("工具流式 SSE 缺少 %q\n--- 完整输出 ---\n%s", want, s)
		}
	}
	// input_json_delta 必须作为 content_block_delta 出现，不能是独立事件
	if strings.Contains(s, "event: input_json_delta") {
		t.Errorf("工具输入增量不应是独立事件\n%s", s)
	}
}

func TestNoAdapterReturnsServiceUnavailable(t *testing.T) {
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return nil, false })
	w := doRequest(t, r, `{"model":"m","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503 (无 adapter 时不应返回 mock)，body=%s", w.Code, w.Body.String())
	}
}

func TestInvalidRequestReturns400(t *testing.T) {
	r := newTestRouter(func() (adapter.StreamProcessor, bool) { return fakeProcessor{}, true })
	// 缺 max_tokens
	w := doRequest(t, r, `{"model":"m","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}
