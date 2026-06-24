package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseSSE 从 FormatSSE 产出的字符串里拆出 event 类型与 data 的 JSON 对象。
// 形如: "event: <type>\ndata: <json>\n\n"
func parseSSE(t *testing.T, s string) (eventType string, data map[string]interface{}) {
	t.Helper()
	for _, ln := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		switch {
		case strings.HasPrefix(ln, "event: "):
			eventType = strings.TrimPrefix(ln, "event: ")
		case strings.HasPrefix(ln, "data: "):
			raw := strings.TrimPrefix(ln, "data: ")
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				t.Fatalf("unmarshal data line failed: %v\nraw: %q\nfull:\n%s", err, raw, s)
			}
			data = m
		}
	}
	if eventType == "" || data == nil {
		t.Fatalf("missing event/data in SSE frame:\n%q", s)
	}
	return eventType, data
}

func TestNewMessageStartEventShape(t *testing.T) {
	ev, data := parseSSE(t, NewMessageStartEvent("msg_123", "claude-opus-4-8"))

	if ev != "message_start" {
		t.Fatalf("event type = %q, want message_start", ev)
	}
	if data["type"] != "message_start" {
		t.Fatalf("data.type = %v, want message_start", data["type"])
	}

	msg, ok := data["message"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.message is not an object: %T", data["message"])
	}

	// usage 初始为 0
	usage, _ := msg["usage"].(map[string]interface{})
	if usage["input_tokens"] != float64(0) || usage["output_tokens"] != float64(0) {
		t.Fatalf("usage = %v, want zeros", usage)
	}

	// content 必须序列化为 []（空数组），而非被丢弃或 null
	content, ok := msg["content"].([]interface{})
	if !ok {
		t.Fatalf("message.content = %T, want [] (got: %v)", msg["content"], msg["content"])
	}
	if len(content) != 0 {
		t.Fatalf("message.content = %v, want empty []", content)
	}

	// stop_reason / stop_sequence 必须显式为 null
	if msg["stop_reason"] != nil {
		t.Fatalf("message.stop_reason = %v, want null", msg["stop_reason"])
	}
	if msg["stop_sequence"] != nil {
		t.Fatalf("message.stop_sequence = %v, want null", msg["stop_sequence"])
	}

	// 防回归：原始字符串里要能看到 "content":[] 与 "stop_reason":null
	raw := NewMessageStartEvent("msg_123", "claude-opus-4-8")
	for _, want := range []string{`"content":[]`, `"stop_reason":null`, `"stop_sequence":null`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("SSE missing %q in:\n%s", want, raw)
		}
	}
}

func TestInputJsonDeltaIsContentBlockDelta(t *testing.T) {
	ev, data := parseSSE(t, NewInputJsonDelta(1, `{"query"`))

	// 关键修复：事件类型必须是 content_block_delta，而不是 input_json_delta
	if ev != "content_block_delta" {
		t.Fatalf("event type = %q, want content_block_delta", ev)
	}
	if data["type"] != "content_block_delta" {
		t.Fatalf("data.type = %v, want content_block_delta", data["type"])
	}
	if data["index"] != float64(1) {
		t.Fatalf("data.index = %v, want 1", data["index"])
	}

	// partial_json 必须嵌在 delta 对象里，且 delta.type = input_json_delta
	delta, ok := data["delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.delta missing/not-object: %v", data["delta"])
	}
	if delta["type"] != "input_json_delta" {
		t.Fatalf("delta.type = %v, want input_json_delta", delta["type"])
	}
	if delta["partial_json"] != `{"query"` {
		t.Fatalf("delta.partial_json = %v, want {\"query\"", delta["partial_json"])
	}

	// 防回归：不能出现独立的 input_json_delta 事件
	raw := NewInputJsonDelta(1, `{"query"`)
	if strings.HasPrefix(raw, "event: input_json_delta") {
		t.Fatalf("regression: emitted standalone input_json_delta event:\n%s", raw)
	}
}

func TestNewContentBlockDeltaEventText(t *testing.T) {
	ev, data := parseSSE(t, NewContentBlockDeltaEvent(0, "hi"))
	if ev != "content_block_delta" {
		t.Fatalf("event type = %q, want content_block_delta", ev)
	}
	delta, _ := data["delta"].(map[string]interface{})
	if delta["type"] != "text_delta" {
		t.Fatalf("delta.type = %v, want text_delta", delta["type"])
	}
	if delta["text"] != "hi" {
		t.Fatalf("delta.text = %v, want hi", delta["text"])
	}
}

func TestExtractUserMessageJoinsText(t *testing.T) {
	// 多个 text 块应拼接成普通字符串，而不是 Go 切片字面量 "[a b]"
	msg := AnthropicMessage{
		Role: "user",
		Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "hello"},
			map[string]interface{}{"type": "text", "text": "world"},
		},
	}
	got := ExtractUserMessage(msg)
	if got != "hello world" {
		t.Fatalf("ExtractUserMessage = %q, want %q", got, "hello world")
	}

	// 纯字符串内容
	if got := ExtractUserMessage(AnthropicMessage{Role: "user", Content: "plain"}); got != "plain" {
		t.Fatalf("ExtractUserMessage(string) = %q, want plain", got)
	}

	// 图片块标记
	imgMsg := AnthropicMessage{
		Role: "user",
		Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "look"},
			map[string]interface{}{"type": "image", "source": map[string]interface{}{"type": "base64"}},
		},
	}
	if got := ExtractUserMessage(imgMsg); !strings.Contains(got, "[Image]") || !strings.Contains(got, "look") {
		t.Fatalf("ExtractUserMessage(img) = %q, want text + [Image]", got)
	}
}

func TestFormatSSEMarshalErrorFallback(t *testing.T) {
	// 无法被 json.Marshal 序列化的值（循环/chan）应降级为 error 事件，而非空 data
	out := FormatSSE("message_start", make(chan int))
	if !strings.HasPrefix(out, "event: error\n") {
		t.Fatalf("expected fallback error event, got:\n%s", out)
	}
	if !strings.Contains(out, `"type":"error"`) {
		t.Fatalf("fallback missing error body:\n%s", out)
	}
}
