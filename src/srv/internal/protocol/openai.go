package protocol

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// OpenAI Chat Completions API
// https://platform.openai.com/docs/api-reference/chat/create
// ============================================================================

// OpenAIRequest OpenAI Chat Completions API 请求
type OpenAIRequest struct {
	Model               string                 `json:"model"`
	Messages            []OpenAIMessage        `json:"messages"`
	MaxTokens           int                    `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                    `json:"max_completion_tokens,omitempty"`
	Temperature         float64                `json:"temperature,omitempty"`
	TopP                float64                `json:"top_p,omitempty"`
	N                   int                    `json:"n,omitempty"`
	Stream              bool                   `json:"stream,omitempty"`
	Stop                interface{}            `json:"stop,omitempty"` // string or []string
	PresencePenalty     float64                `json:"presence_penalty,omitempty"`
	FrequencyPenalty    float64                `json:"frequency_penalty,omitempty"`
	LogitBias           map[string]float64     `json:"logit_bias,omitempty"`
	User                string                 `json:"user,omitempty"`
	ResponseFormat      map[string]interface{} `json:"response_format,omitempty"`
	Seed                int                    `json:"seed,omitempty"`
	Tools               []Tool                 `json:"tools,omitempty"`
	ToolChoice          interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls   *bool                  `json:"parallel_tool_calls,omitempty"`
	Logprobs            *bool                  `json:"logprobs,omitempty"`
	TopLogprobs         int                    `json:"top_logprobs,omitempty"`
	Functions           []OpenAIFunction       `json:"functions,omitempty"`     // legacy chat-completions tools
	FunctionCall        interface{}            `json:"function_call,omitempty"` // legacy "auto" | "none" | {"name":...}
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Store               *bool                  `json:"store,omitempty"`
	Modalities          []string               `json:"modalities,omitempty"`
	ReasoningEffort     string                 `json:"reasoning_effort,omitempty"`
	ServiceTier         string                 `json:"service_tier,omitempty"`
}

// OpenAIMessage 消息
type OpenAIMessage struct {
	Role         string                 `json:"role"`              // "developer" | "system" | "user" | "assistant" | "tool" | "function"
	Content      interface{}            `json:"content,omitempty"` // string or []ContentPart
	Name         string                 `json:"name,omitempty"`
	ToolCalls    []ToolCall             `json:"tool_calls,omitempty"`
	ToolCallID   string                 `json:"tool_call_id,omitempty"`
	FunctionCall map[string]interface{} `json:"function_call,omitempty"` // legacy assistant function call
	Refusal      string                 `json:"refusal,omitempty"`
}

// ContentPart 内容部分（支持文本和图片）
type ContentPart struct {
	Type     string    `json:"type"` // "text" | "image_url" | "input_text" | "input_image"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// UnmarshalJSON accepts both documented image_url object form and common
// client shorthand where image_url is sent as a string.
func (p *ContentPart) UnmarshalJSON(data []byte) error {
	var raw struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		ImageURL json.RawMessage `json:"image_url"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Type = raw.Type
	p.Text = raw.Text
	if len(raw.ImageURL) > 0 && string(raw.ImageURL) != "null" {
		var url string
		if err := json.Unmarshal(raw.ImageURL, &url); err == nil {
			p.ImageURL = &ImageURL{URL: url}
			return nil
		}
		var image ImageURL
		if err := json.Unmarshal(raw.ImageURL, &image); err != nil {
			return err
		}
		p.ImageURL = &image
	}
	return nil
}

// ImageURL 图片 URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto" | "low" | "high"
}

// Tool 工具定义
type Tool struct {
	Type     string                 `json:"type"` // "function"
	Function map[string]interface{} `json:"function"`
}

// OpenAIFunction is the legacy Chat Completions function declaration shape.
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	Index    *int                   `json:"index,omitempty"` // 流式响应中使用
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"` // "function"
	Function map[string]interface{} `json:"function,omitempty"`
}

// Validate 验证请求
func (r *OpenAIRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}

	if len(r.Messages) == 0 {
		return fmt.Errorf("messages is required and must not be empty")
	}

	// 验证消息格式
	for i, msg := range r.Messages {
		if msg.Role != "developer" && msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" && msg.Role != "tool" && msg.Role != "function" {
			return fmt.Errorf("message[%d]: role must be 'developer', 'system', 'user', 'assistant', 'tool', or 'function'", i)
		}

		if msg.Content == nil && len(msg.ToolCalls) == 0 && msg.ToolCallID == "" && msg.FunctionCall == nil {
			return fmt.Errorf("message[%d]: content, tool_calls, tool_call_id, or function_call is required", i)
		}
	}

	return nil
}

// ============================================================================
// Response (非流式)
// ============================================================================

// OpenAIResponse OpenAI Chat Completions API 响应
type OpenAIResponse struct {
	ID                string      `json:"id"`
	Object            string      `json:"object"` // "chat.completion"
	Created           int64       `json:"created"`
	Model             string      `json:"model"`
	Choices           []Choice    `json:"choices"`
	Usage             OpenAIUsage `json:"usage"`
	SystemFingerprint string      `json:"system_fingerprint,omitempty"`
}

// Choice 选择
type Choice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"` // "stop" | "length" | "tool_calls" | "content_filter"
	Logprobs     *Logprobs     `json:"logprobs,omitempty"`
}

// Logprobs token 概率信息
type Logprobs struct {
	Content []TokenLogprob `json:"content,omitempty"`
}

// TokenLogprob 单个 token 的概率信息
type TokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

// OpenAIUsage token 使用情况
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ============================================================================
// Streaming Response
// ============================================================================

// OpenAIStreamResponse 流式响应块
type OpenAIStreamResponse struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"` // "chat.completion.chunk"
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	SystemFingerprint string         `json:"system_fingerprint,omitempty"`
}

// StreamChoice 流式选择
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        DeltaMessage `json:"delta"`
	FinishReason *string      `json:"finish_reason"` // null or "stop" | "length" | "tool_calls"
	Logprobs     interface{}  `json:"logprobs,omitempty"`
}

// DeltaMessage 增量消息
type DeltaMessage struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ============================================================================
// Helpers
// ============================================================================

// FormatOpenAISSE 格式化为 OpenAI SSE 格式
func FormatOpenAISSE(chunk *OpenAIStreamResponse) string {
	jsonData, _ := json.Marshal(chunk)
	return fmt.Sprintf("data: %s\n\n", string(jsonData))
}

// NewOpenAIStreamChunk 创建流式响应块
func NewOpenAIStreamChunk(id, model string, created int64, content string, finishReason *string) string {
	chunk := &OpenAIStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: DeltaMessage{
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}
	return FormatOpenAISSE(chunk)
}

// NewOpenAIStreamStart 创建流式开始块（包含 role）
func NewOpenAIStreamStart(id, model string, created int64) string {
	chunk := &OpenAIStreamResponse{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: DeltaMessage{
					Role: "assistant",
				},
				FinishReason: nil,
			},
		},
	}
	return FormatOpenAISSE(chunk)
}

// NewOpenAIStreamDone 创建流式结束标记
func NewOpenAIStreamDone() string {
	return "data: [DONE]\n\n"
}

// ============================================================================
// Error Response
// ============================================================================

// OpenAIError OpenAI 错误响应
type OpenAIError struct {
	Error OpenAIErrorDetail `json:"error"`
}

// OpenAIErrorDetail 错误详情
type OpenAIErrorDetail struct {
	Message string      `json:"message"`
	Type    string      `json:"type"`
	Param   interface{} `json:"param,omitempty"`
	Code    interface{} `json:"code,omitempty"`
}

// NewOpenAIError 创建错误响应
func NewOpenAIError(message, errorType string) OpenAIError {
	return OpenAIError{
		Error: OpenAIErrorDetail{
			Message: message,
			Type:    errorType,
		},
	}
}
