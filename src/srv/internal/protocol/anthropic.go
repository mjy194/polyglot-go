package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================================
// Anthropic Messages API
// https://docs.anthropic.com/claude/reference/messages_post
// ============================================================================

// AnthropicRequest Anthropic Messages API 请求
type AnthropicRequest struct {
	Model         string                 `json:"model"`
	Messages      []AnthropicMessage     `json:"messages"`
	MaxTokens     int                    `json:"max_tokens"`
	System        interface{}            `json:"system,omitempty"` // string or []SystemBlock
	Stream        bool                   `json:"stream,omitempty"`
	Temperature   float64                `json:"temperature,omitempty"`
	TopP          float64                `json:"top_p,omitempty"`
	TopK          int                    `json:"top_k,omitempty"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Tools         []AnthropicTool        `json:"tools,omitempty"`
	ToolChoice    interface{}            `json:"tool_choice,omitempty"` // "auto" | "any" | {"type":"tool","name":"..."}
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AnthropicMessage 消息
type AnthropicMessage struct {
	Role    string                   `json:"role"` // "user" | "assistant"
	Content interface{}              `json:"content"` // string or []ContentBlock
}

// ContentBlock 内容块（支持文本、图片、工具使用、工具结果）
type ContentBlock struct {
	Type   string       `json:"type"` // "text" | "image" | "tool_use" | "tool_result"
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`

	// Tool Use
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`

	// Tool Result
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or []ContentBlock
	IsError   bool        `json:"is_error,omitempty"`

	// Prompt caching（可作用于 message 内容块，如 system/tool_result/text）
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// AnthropicTool 工具定义
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// SystemBlock 系统消息块（支持文本和缓存控制）
type SystemBlock struct {
	Type         string        `json:"type"` // "text"
	Text         string        `json:"text"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl 提示词缓存控制
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ImageSource 图片源
type ImageSource struct {
	Type      string `json:"type"` // "base64" | "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

// Validate 验证请求
func (r *AnthropicRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}

	if len(r.Messages) == 0 {
		return fmt.Errorf("messages is required and must not be empty")
	}

	if r.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be greater than 0")
	}

	// 验证消息格式
	for i, msg := range r.Messages {
		switch msg.Role {
		case "user", "assistant", "system": // system: mid-conversation system 消息（真实 API 在 Opus 4.8 等模型上接受）
		default:
			return fmt.Errorf("message[%d]: role must be 'user', 'assistant', or 'system' (got %q)", i, msg.Role)
		}

		if msg.Content == nil {
			return fmt.Errorf("message[%d]: content is required", i)
		}
	}

	return nil
}

// ============================================================================
// Response (非流式)
// ============================================================================

// AnthropicResponse Anthropic Messages API 响应
type AnthropicResponse struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // "message"
	Role         string                 `json:"role"` // "assistant"
	Content      []ResponseContentBlock `json:"content"`
	Model        string                 `json:"model"`
	StopReason   string                 `json:"stop_reason,omitempty"`   // "end_turn" | "max_tokens" | "stop_sequence"
	StopSequence string                 `json:"stop_sequence,omitempty"`
	Usage        Usage                  `json:"usage"`
}

// ResponseContentBlock 响应内容块
type ResponseContentBlock struct {
	Type string `json:"type"` // "text" | "tool_use"
	Text string `json:"text,omitempty"`

	// Tool Use 字段
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// Usage token 使用情况
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ============================================================================
// Streaming Events
// ============================================================================

// StreamEvent 流式事件基础结构
type StreamEvent struct {
	Type string `json:"type"`
}

// MessageStartEvent message_start 事件
type MessageStartEvent struct {
	Type    string           `json:"type"` // "message_start"
	Message MessageStartData `json:"message"`
}

type MessageStartData struct {
	ID     string `json:"id"`
	Type   string `json:"type"` // "message"
	Role   string `json:"role"` // "assistant"
	Model  string `json:"model"`
	Usage  Usage  `json:"usage"`

	// 真实协议在 message_start 中即给出这些字段（初始为 null / 空数组），
	// 用指针/切片零值使其序列化为 null / []，避免被 omitempty 丢弃。
	Content      []ResponseContentBlock `json:"content"`
	StopReason   *string                `json:"stop_reason"`
	StopSequence *string                `json:"stop_sequence"`
}

// ContentBlockStartEvent content_block_start 事件
type ContentBlockStartEvent struct {
	Type         string       `json:"type"` // "content_block_start"
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// ContentBlockDeltaEvent content_block_delta 事件
type ContentBlockDeltaEvent struct {
	Type  string      `json:"type"` // "content_block_delta"
	Index int         `json:"index"`
	Delta DeltaBlock  `json:"delta"`
}

// DeltaBlock 增量数据（text_delta 与 input_json_delta 共用此结构）
type DeltaBlock struct {
	Type        string `json:"type"` // "text_delta" | "input_json_delta"
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
}

// ContentBlockStopEvent content_block_stop 事件
type ContentBlockStopEvent struct {
	Type  string `json:"type"` // "content_block_stop"
	Index int    `json:"index"`
}

// MessageDeltaEvent message_delta 事件
type MessageDeltaEvent struct {
	Type  string      `json:"type"` // "message_delta"
	Delta MessageDelta `json:"delta"`
	Usage Usage       `json:"usage"`
}

type MessageDelta struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// MessageStopEvent message_stop 事件
type MessageStopEvent struct {
	Type string `json:"type"` // "message_stop"
}

// ContentBlockStartEventWithToolUse content_block_start 事件（工具使用）
type ContentBlockStartEventWithToolUse struct {
	Type         string       `json:"type"` // "content_block_start"
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

// InputJsonDeltaEvent input_json_delta 事件（工具使用的输入增量）
type InputJsonDeltaEvent struct {
	Type       string `json:"type"` // "input_json_delta"
	Index      int    `json:"index"`
	PartialJSON string `json:"partial_json"`
}

// PingEvent ping 事件
type PingEvent struct {
	Type string `json:"type"` // "ping"
}

// ErrorEvent error 事件
type ErrorEvent struct {
	Type  string     `json:"type"` // "error"
	Error ErrorBlock `json:"error"`
}

type ErrorBlock struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ============================================================================
// Helpers
// ============================================================================

// FormatSSE 格式化为 SSE (Server-Sent Events) 格式
func FormatSSE(eventType string, data interface{}) string {
	jsonData, err := json.Marshal(data)
	if err != nil {
		// 序列化失败时不静默产出空 data，降级为 error 事件，便于客户端/调试察觉
		errEv := ErrorEvent{
			Type:  "error",
			Error: ErrorBlock{Type: "internal_error", Message: fmt.Sprintf("failed to marshal %s event: %v", eventType, err)},
		}
		fallback, _ := json.Marshal(errEv)
		return fmt.Sprintf("event: error\ndata: %s\n\n", string(fallback))
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, string(jsonData))
}

// NewMessageStartEvent 创建 message_start 事件
func NewMessageStartEvent(messageID, model string) string {
	event := MessageStartEvent{
		Type: "message_start",
		Message: MessageStartData{
			ID:           messageID,
			Type:         "message",
			Role:         "assistant",
			Model:        model,
			Usage:        Usage{InputTokens: 0, OutputTokens: 0},
			Content:      []ResponseContentBlock{}, // -> "content":[]
			StopReason:   nil,                      // -> "stop_reason":null
			StopSequence: nil,                      // -> "stop_sequence":null
		},
	}
	return FormatSSE("message_start", event)
}

// NewContentBlockStartEvent 创建 content_block_start 事件
func NewContentBlockStartEvent(index int) string {
	event := ContentBlockStartEvent{
		Type:  "content_block_start",
		Index: index,
		ContentBlock: ContentBlock{
			Type: "text",
			Text: "",
		},
	}
	return FormatSSE("content_block_start", event)
}

// NewContentBlockDeltaEvent 创建 content_block_delta 事件
func NewContentBlockDeltaEvent(index int, text string) string {
	event := ContentBlockDeltaEvent{
		Type:  "content_block_delta",
		Index: index,
		Delta: DeltaBlock{
			Type: "text_delta",
			Text: text,
		},
	}
	return FormatSSE("content_block_delta", event)
}

// NewContentBlockStopEvent 创建 content_block_stop 事件
func NewContentBlockStopEvent(index int) string {
	event := ContentBlockStopEvent{
		Type:  "content_block_stop",
		Index: index,
	}
	return FormatSSE("content_block_stop", event)
}

// NewMessageDeltaEvent 创建 message_delta 事件
func NewMessageDeltaEvent(stopReason string, inputTokens, outputTokens int) string {
	event := MessageDeltaEvent{
		Type: "message_delta",
		Delta: MessageDelta{
			StopReason: stopReason,
		},
		Usage: Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}
	return FormatSSE("message_delta", event)
}

// NewMessageStopEvent 创建 message_stop 事件
func NewMessageStopEvent() string {
	event := MessageStopEvent{
		Type: "message_stop",
	}
	return FormatSSE("message_stop", event)
}

// NewToolUseBlockStart 创建工具使用开始事件
func NewToolUseBlockStart(index int, id, name string) string {
	event := ContentBlockStartEventWithToolUse{
		Type:  "content_block_start",
		Index: index,
		ContentBlock: ContentBlock{
			Type: "tool_use",
			ID:   id,
			Name: name,
		},
	}
	return FormatSSE("content_block_start", event)
}

// NewInputJsonDelta 创建工具输入增量事件
//
// 注意：真实 Anthropic 协议中，工具输入增量属于 content_block_delta 事件，
// 其 delta.type 为 "input_json_delta"（而非独立的 input_json_delta 事件），
// 形如:
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","index":N,"delta":{"type":"input_json_delta","partial_json":"..."}}
func NewInputJsonDelta(index int, partialJSON string) string {
	event := ContentBlockDeltaEvent{
		Type:  "content_block_delta",
		Index: index,
		Delta: DeltaBlock{
			Type:        "input_json_delta",
			PartialJSON: partialJSON,
		},
	}
	return FormatSSE("content_block_delta", event)
}

// NewPingEvent 创建 ping 事件（保活/心跳）
func NewPingEvent() string {
	return FormatSSE("ping", PingEvent{Type: "ping"})
}

// NewErrorEvent 创建 error 事件（流中途出错时发送）
func NewErrorEvent(errorType, message string) string {
	return FormatSSE("error", ErrorEvent{
		Type:  "error",
		Error: ErrorBlock{Type: errorType, Message: message},
	})
}

// ExtractUserMessage 提取用户消息文本（辅助函数）
func ExtractUserMessage(msg AnthropicMessage) string {
	// 处理字符串类型
	if str, ok := msg.Content.(string); ok {
		return str
	}

	// 处理 ContentBlock 数组（通过 JSON 解析）
	if blocks, ok := msg.Content.([]interface{}); ok {
		var textParts []string
		for _, block := range blocks {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				} else if blockMap["type"] == "image" {
					// 识别图片
					textParts = append(textParts, "[Image]")
				}
			}
		}
		if len(textParts) > 0 {
			return strings.Join(textParts, " ")
		}
	}

	return ""
}
