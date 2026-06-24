package protocol

import (
	"encoding/json"
	"fmt"
)

// ============================================================================
// Google Gemini API
// https://ai.google.dev/api/rest/v1/models/generateContent
// ============================================================================

// GeminiRequest Gemini API 请求
type GeminiRequest struct {
	Model             string                  `json:"model,omitempty"`
	Contents          []GeminiContent         `json:"contents"`
	Tools             []GeminiTool            `json:"tools,omitempty"`
	ToolConfig        *GeminiToolConfig       `json:"toolConfig,omitempty"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
}

// UnmarshalJSON accepts both native Gemini lowerCamelCase JSON and snake_case
// variants produced by some SDK shims/CLI clients.
func (r *GeminiRequest) UnmarshalJSON(data []byte) error {
	type alias GeminiRequest
	var raw struct {
		alias
		ToolConfigSnake        *GeminiToolConfig       `json:"tool_config"`
		SafetySettingsSnake    []GeminiSafetySetting   `json:"safety_settings"`
		GenerationConfigSnake  *GeminiGenerationConfig `json:"generation_config"`
		SystemInstructionSnake *GeminiContent          `json:"system_instruction"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = GeminiRequest(raw.alias)
	if r.ToolConfig == nil {
		r.ToolConfig = raw.ToolConfigSnake
	}
	if len(r.SafetySettings) == 0 {
		r.SafetySettings = raw.SafetySettingsSnake
	}
	if r.GenerationConfig == nil {
		r.GenerationConfig = raw.GenerationConfigSnake
	}
	if r.SystemInstruction == nil {
		r.SystemInstruction = raw.SystemInstructionSnake
	}
	return nil
}

// GeminiContent 内容
type GeminiContent struct {
	Role  string       `json:"role,omitempty"` // "user" | "model" | "function"
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart 内容部分
type GeminiPart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *GeminiInlineData       `json:"inlineData,omitempty"`
	FileData         *GeminiFileData         `json:"fileData,omitempty"`
	FunctionCall     *GeminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *GeminiFunctionResponse `json:"functionResponse,omitempty"`
}

func (p *GeminiPart) UnmarshalJSON(data []byte) error {
	type alias GeminiPart
	var raw struct {
		alias
		InlineDataSnake       *GeminiInlineData       `json:"inline_data"`
		FileDataSnake         *GeminiFileData         `json:"file_data"`
		FunctionCallSnake     *GeminiFunctionCall     `json:"function_call"`
		FunctionResponseSnake *GeminiFunctionResponse `json:"function_response"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*p = GeminiPart(raw.alias)
	if p.InlineData == nil {
		p.InlineData = raw.InlineDataSnake
	}
	if p.FileData == nil {
		p.FileData = raw.FileDataSnake
	}
	if p.FunctionCall == nil {
		p.FunctionCall = raw.FunctionCallSnake
	}
	if p.FunctionResponse == nil {
		p.FunctionResponse = raw.FunctionResponseSnake
	}
	return nil
}

// GeminiInlineData 内联数据（图片等）
type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

func (d *GeminiInlineData) UnmarshalJSON(data []byte) error {
	type alias GeminiInlineData
	var raw struct {
		alias
		MimeTypeSnake string `json:"mime_type"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*d = GeminiInlineData(raw.alias)
	if d.MimeType == "" {
		d.MimeType = raw.MimeTypeSnake
	}
	return nil
}

// GeminiFileData represents Gemini fileData/file_data parts.
type GeminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri,omitempty"`
}

func (d *GeminiFileData) UnmarshalJSON(data []byte) error {
	type alias GeminiFileData
	var raw struct {
		alias
		MimeTypeSnake string `json:"mime_type"`
		FileURISnake  string `json:"file_uri"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*d = GeminiFileData(raw.alias)
	if d.MimeType == "" {
		d.MimeType = raw.MimeTypeSnake
	}
	if d.FileURI == "" {
		d.FileURI = raw.FileURISnake
	}
	return nil
}

// GeminiFunctionCall 函数调用
type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// GeminiFunctionResponse 函数响应
type GeminiFunctionResponse struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// GeminiTool 工具定义
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

func (t *GeminiTool) UnmarshalJSON(data []byte) error {
	type alias GeminiTool
	var raw struct {
		alias
		FunctionDeclarationsSnake []GeminiFunctionDeclaration `json:"function_declarations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = GeminiTool(raw.alias)
	if len(t.FunctionDeclarations) == 0 {
		t.FunctionDeclarations = raw.FunctionDeclarationsSnake
	}
	return nil
}

// GeminiFunctionDeclaration 函数声明
type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// GeminiToolConfig 工具配置
type GeminiToolConfig struct {
	FunctionCallingConfig *GeminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

func (c *GeminiToolConfig) UnmarshalJSON(data []byte) error {
	type alias GeminiToolConfig
	var raw struct {
		alias
		FunctionCallingConfigSnake *GeminiFunctionCallingConfig `json:"function_calling_config"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = GeminiToolConfig(raw.alias)
	if c.FunctionCallingConfig == nil {
		c.FunctionCallingConfig = raw.FunctionCallingConfigSnake
	}
	return nil
}

// GeminiFunctionCallingConfig 函数调用配置
type GeminiFunctionCallingConfig struct {
	Mode string `json:"mode,omitempty"` // "AUTO" | "ANY" | "NONE"
}

// GeminiGenerationConfig 生成配置
type GeminiGenerationConfig struct {
	Temperature        float64                `json:"temperature,omitempty"`
	TopP               float64                `json:"topP,omitempty"`
	TopK               int                    `json:"topK,omitempty"`
	MaxOutputTokens    int                    `json:"maxOutputTokens,omitempty"`
	StopSequences      []string               `json:"stopSequences,omitempty"`
	ResponseMimeType   string                 `json:"responseMimeType,omitempty"`
	ResponseSchema     map[string]interface{} `json:"responseSchema,omitempty"`
	ResponseJsonSchema map[string]interface{} `json:"responseJsonSchema,omitempty"`
}

func (c *GeminiGenerationConfig) UnmarshalJSON(data []byte) error {
	type alias GeminiGenerationConfig
	var raw struct {
		alias
		TopPSnake               float64                `json:"top_p"`
		TopKSnake               int                    `json:"top_k"`
		MaxOutputTokensSnake    int                    `json:"max_output_tokens"`
		StopSequencesSnake      []string               `json:"stop_sequences"`
		ResponseMimeTypeSnake   string                 `json:"response_mime_type"`
		ResponseSchemaSnake     map[string]interface{} `json:"response_schema"`
		ResponseJsonSchemaSnake map[string]interface{} `json:"response_json_schema"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = GeminiGenerationConfig(raw.alias)
	if c.TopP == 0 {
		c.TopP = raw.TopPSnake
	}
	if c.TopK == 0 {
		c.TopK = raw.TopKSnake
	}
	if c.MaxOutputTokens == 0 {
		c.MaxOutputTokens = raw.MaxOutputTokensSnake
	}
	if len(c.StopSequences) == 0 {
		c.StopSequences = raw.StopSequencesSnake
	}
	if c.ResponseMimeType == "" {
		c.ResponseMimeType = raw.ResponseMimeTypeSnake
	}
	if c.ResponseSchema == nil {
		c.ResponseSchema = raw.ResponseSchemaSnake
	}
	if c.ResponseJsonSchema == nil {
		c.ResponseJsonSchema = raw.ResponseJsonSchemaSnake
	}
	return nil
}

// GeminiSafetySetting 安全设置
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// Validate 验证请求
func (r *GeminiRequest) Validate() error {
	if len(r.Contents) == 0 {
		return fmt.Errorf("contents is required and must not be empty")
	}

	// 验证每个 content
	for i, content := range r.Contents {
		if len(content.Parts) == 0 {
			return fmt.Errorf("contents[%d]: parts is required and must not be empty", i)
		}
	}

	return nil
}

// ============================================================================
// Response (非流式)
// ============================================================================

// GeminiResponse Gemini API 响应
type GeminiResponse struct {
	Candidates     []GeminiCandidate     `json:"candidates"`
	PromptFeedback *GeminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *GeminiUsageMetadata  `json:"usageMetadata,omitempty"`
}

// GeminiCandidate 候选结果
type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason,omitempty"` // "STOP" | "MAX_TOKENS" | "SAFETY" | "OTHER"
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// GeminiSafetyRating 安全评级
type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// GeminiPromptFeedback 提示反馈
type GeminiPromptFeedback struct {
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// GeminiUsageMetadata 使用元数据
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// ============================================================================
// Streaming Response
// ============================================================================

// GeminiStreamResponse 流式响应
type GeminiStreamResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// ============================================================================
// Helpers
// ============================================================================

// FormatGeminiSSE 格式化为 Gemini SSE 格式
func FormatGeminiSSE(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("data: %s\n\n", string(jsonData))
}

// NewGeminiStreamChunk 创建流式响应块
func NewGeminiStreamChunk(text string, finishReason string) string {
	response := &GeminiStreamResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{
							Text: text,
						},
					},
				},
				FinishReason: finishReason,
				Index:        0,
			},
		},
	}
	return FormatGeminiSSE(response)
}

// NewGeminiFunctionCallChunk 创建函数调用流式响应块
func NewGeminiFunctionCallChunk(name string, args map[string]interface{}, finishReason string) string {
	response := &GeminiStreamResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Role: "model",
					Parts: []GeminiPart{
						{
							FunctionCall: &GeminiFunctionCall{
								Name: name,
								Args: args,
							},
						},
					},
				},
				FinishReason: finishReason,
				Index:        0,
			},
		},
	}
	return FormatGeminiSSE(response)
}

// ExtractGeminiUserMessage 提取用户消息文本
func ExtractGeminiUserMessage(contents []GeminiContent) (string, bool) {
	hasImage := false
	var textParts []string

	for i := len(contents) - 1; i >= 0; i-- {
		if contents[i].Role == "user" {
			for _, part := range contents[i].Parts {
				if part.Text != "" {
					textParts = append(textParts, part.Text)
				} else if part.InlineData != nil {
					hasImage = true
					textParts = append(textParts, "[Image]")
				}
			}
			break
		}
	}

	if len(textParts) > 0 {
		return fmt.Sprintf("%v", textParts), hasImage
	}
	return "", hasImage
}
