package protocol

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// OpenAI Responses API (/v1/responses)
// https://platform.openai.com/docs/api-reference/responses
// codex CLI 走的就是这个 API（它已不再支持 chat completions wire_api）。
// ============================================================================

// ResponseRequest Responses API 请求。Input/Instructions 用 RawMessage 保留 string 或数组两种形态。
type ResponseRequest struct {
	Model              string                 `json:"model"`
	Input              json.RawMessage        `json:"input"`                  // string 或 []ResponseInputItem
	Instructions       json.RawMessage        `json:"instructions,omitempty"` // string 或 []{type:"text",text}
	Stream             bool                   `json:"stream,omitempty"`
	MaxOutputTokens    int                    `json:"max_output_tokens,omitempty"`
	Temperature        *float64               `json:"temperature,omitempty"`
	TopP               *float64               `json:"top_p,omitempty"`
	Tools              []ResponseTool         `json:"tools,omitempty"`
	ToolChoice         interface{}            `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool                  `json:"parallel_tool_calls,omitempty"`
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	Include            []string               `json:"include,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	Reasoning          map[string]interface{} `json:"reasoning,omitempty"`
	Text               *ResponseTextConfig    `json:"text,omitempty"`
	ResponseFormat     interface{}            `json:"response_format,omitempty"` // legacy compatibility
	Store              *bool                  `json:"store,omitempty"`
	Truncation         string                 `json:"truncation,omitempty"`
	User               string                 `json:"user,omitempty"`
	TopLogprobs        int                    `json:"top_logprobs,omitempty"`
	MaxToolCalls       int                    `json:"max_tool_calls,omitempty"`
}

// ResponseInputItem input 数组中的一项
type ResponseInputItem struct {
	Type    string          `json:"type,omitempty"`    // "message" | "function_call" | "function_call_output"
	Role    string          `json:"role,omitempty"`    // "user"|"assistant"|"system"|"developer"
	Content json.RawMessage `json:"content,omitempty"` // string 或 []ResponseContentPart
	ID      string          `json:"id,omitempty"`
	// function_call / function_call_output（多轮工具用；当前 adapter 不产生工具调用，仅尽量透传）
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"` // string or structured JSON
}

// ResponseContentPart 内容块
type ResponseContentPart struct {
	Type     string      `json:"type"` // "input_text" | "output_text" | "text" | "input_image" | "input_file"
	Text     string      `json:"text,omitempty"`
	ImageURL interface{} `json:"image_url,omitempty"` // string or {url,detail}
	FileID   string      `json:"file_id,omitempty"`
	FileData string      `json:"file_data,omitempty"`
	Filename string      `json:"filename,omitempty"`
	Detail   string      `json:"detail,omitempty"`
}

// ImageURLValue normalizes Responses input_image image_url string/object forms.
func (p ResponseContentPart) ImageURLValue() (url string, detail string) {
	switch v := p.ImageURL.(type) {
	case string:
		return v, p.Detail
	case map[string]interface{}:
		if s, ok := v["url"].(string); ok {
			url = s
		}
		if d, ok := v["detail"].(string); ok {
			detail = d
		}
		if detail == "" {
			detail = p.Detail
		}
	}
	return url, detail
}

// ResponseTool Responses API 的工具定义（{type:"function", name, description, parameters}）
type ResponseTool struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ResponseTextConfig covers Responses API structured output controls.
type ResponseTextConfig struct {
	Format *ResponseTextFormat `json:"format,omitempty"`
}

type ResponseTextFormat struct {
	Type        string                 `json:"type,omitempty"` // "text" | "json_object" | "json_schema"
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Strict      *bool                  `json:"strict,omitempty"`
}

// Validate 校验请求
func (r *ResponseRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}
	if len(r.Input) == 0 || isNullOrBlankString(r.Input) {
		return fmt.Errorf("input is required")
	}
	return nil
}

// ============================================================================
// Response (非流式)
// ============================================================================

// ResponseResponse Responses API 响应
type ResponseResponse struct {
	ID        string               `json:"id"`     // "resp_..."
	Object    string               `json:"object"` // "response"
	Model     string               `json:"model"`
	CreatedAt float64              `json:"created_at,omitempty"`
	Status    string               `json:"status"` // "completed" | "in_progress" | "failed"
	Output    []ResponseOutputItem `json:"output"`
	Usage     ResponseUsage        `json:"usage"`
}

// ResponseOutputItem 输出项
type ResponseOutputItem struct {
	Type   string `json:"type"` // "message" | "function_call"
	ID     string `json:"id"`   // "msg_..." | "fc_..."
	Role   string `json:"role,omitempty"`
	Status string `json:"status,omitempty"` // "completed" | "in_progress"
	// message（OpenAI 即使为空也输出 "content":[]，去掉 omitempty）
	Content []ResponseOutputContent `json:"content"`
	// function_call
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ResponseOutputContent 输出内容块
type ResponseOutputContent struct {
	Type        string        `json:"type"` // "output_text"
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"` // OpenAI 总是带 []；codex 的严格反序列化需要
}

// ResponseUsage token 用量
type ResponseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ============================================================================
// Helpers
// ============================================================================

// FormatResponseSSE 格式化为 Responses API 的 SSE 事件（event: <type>\ndata: <json>\n\n）
func FormatResponseSSE(event string, data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(jsonData))
}

// isNullOrBlankString 判断 RawMessage 是否为 null 或空字符串
func isNullOrBlankString(raw json.RawMessage) bool {
	s := string(raw)
	return s == "null" || s == "\"\""
}
