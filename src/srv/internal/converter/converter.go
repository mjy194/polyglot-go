package converter

import (
	"fmt"
	"polyglot/internal/protocol"
)

// ============================================================================
// Protocol Converter
// 在不同 AI API 协议之间转换
// ============================================================================

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolAnthropic ProtocolType = "anthropic"
	ProtocolOpenAI    ProtocolType = "openai"
	ProtocolGemini    ProtocolType = "gemini"
)

// ConversionError 转换错误
type ConversionError struct {
	From    ProtocolType
	To      ProtocolType
	Message string
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("conversion error from %s to %s: %s", e.From, e.To, e.Message)
}

// ============================================================================
// Model Mapping
// ============================================================================

// ModelMapper 模型映射器
type ModelMapper struct {
	mappings map[string]map[string]string // from_protocol -> model -> target_model
}

// NewModelMapper 创建模型映射器
func NewModelMapper() *ModelMapper {
	return &ModelMapper{
		mappings: map[string]map[string]string{
			// OpenAI -> Anthropic
			"openai_to_anthropic": {
				"gpt-4":              "claude-opus-4-8",
				"gpt-4-turbo":        "claude-opus-4-8",
				"gpt-4o":             "claude-opus-4-8",
				"gpt-3.5-turbo":      "claude-sonnet-4-6",
				"gpt-4-vision-preview": "claude-opus-4-8",
			},
			// Anthropic -> OpenAI
			"anthropic_to_openai": {
				"claude-opus-4-8":   "gpt-4",
				"claude-sonnet-4-6": "gpt-3.5-turbo",
				"claude-haiku-4-5":  "gpt-3.5-turbo",
			},
			// Gemini -> Anthropic
			"gemini_to_anthropic": {
				"gemini-pro":        "claude-opus-4-8",
				"gemini-pro-vision": "claude-opus-4-8",
			},
			// Gemini -> OpenAI
			"gemini_to_openai": {
				"gemini-pro":        "gpt-4",
				"gemini-pro-vision": "gpt-4-vision-preview",
			},
		},
	}
}

// Map 映射模型名称
func (m *ModelMapper) Map(from, to ProtocolType, model string) string {
	key := fmt.Sprintf("%s_to_%s", from, to)
	if mapping, ok := m.mappings[key]; ok {
		if mapped, ok := mapping[model]; ok {
			return mapped
		}
	}
	// 如果没有映射，返回原模型名
	return model
}

// ============================================================================
// Request Converters
// ============================================================================

// ConvertRequest 转换请求
func ConvertRequest(from, to ProtocolType, req interface{}) (interface{}, error) {
	switch from {
	case ProtocolAnthropic:
		return convertFromAnthropic(to, req)
	case ProtocolOpenAI:
		return convertFromOpenAI(to, req)
	case ProtocolGemini:
		return convertFromGemini(to, req)
	default:
		return nil, &ConversionError{From: from, To: to, Message: "unsupported source protocol"}
	}
}

// convertFromAnthropic 从 Anthropic 格式转换
func convertFromAnthropic(to ProtocolType, req interface{}) (interface{}, error) {
	anthropicReq, ok := req.(*protocol.AnthropicRequest)
	if !ok {
		return nil, &ConversionError{From: ProtocolAnthropic, To: to, Message: "invalid request type"}
	}

	switch to {
	case ProtocolOpenAI:
		return anthropicToOpenAI(anthropicReq)
	case ProtocolGemini:
		return anthropicToGemini(anthropicReq)
	case ProtocolAnthropic:
		return anthropicReq, nil // 无需转换
	default:
		return nil, &ConversionError{From: ProtocolAnthropic, To: to, Message: "unsupported target protocol"}
	}
}

// convertFromOpenAI 从 OpenAI 格式转换
func convertFromOpenAI(to ProtocolType, req interface{}) (interface{}, error) {
	openaiReq, ok := req.(*protocol.OpenAIRequest)
	if !ok {
		return nil, &ConversionError{From: ProtocolOpenAI, To: to, Message: "invalid request type"}
	}

	switch to {
	case ProtocolAnthropic:
		return openaiToAnthropic(openaiReq)
	case ProtocolGemini:
		return openaiToGemini(openaiReq)
	case ProtocolOpenAI:
		return openaiReq, nil // 无需转换
	default:
		return nil, &ConversionError{From: ProtocolOpenAI, To: to, Message: "unsupported target protocol"}
	}
}

// convertFromGemini 从 Gemini 格式转换
func convertFromGemini(to ProtocolType, req interface{}) (interface{}, error) {
	geminiReq, ok := req.(*protocol.GeminiRequest)
	if !ok {
		return nil, &ConversionError{From: ProtocolGemini, To: to, Message: "invalid request type"}
	}

	switch to {
	case ProtocolAnthropic:
		return geminiToAnthropic(geminiReq)
	case ProtocolOpenAI:
		return geminiToOpenAI(geminiReq)
	case ProtocolGemini:
		return geminiReq, nil // 无需转换
	default:
		return nil, &ConversionError{From: ProtocolGemini, To: to, Message: "unsupported target protocol"}
	}
}

// ============================================================================
// Response Converters
// ============================================================================

// ConvertResponse 转换响应
func ConvertResponse(from, to ProtocolType, resp interface{}) (interface{}, error) {
	switch from {
	case ProtocolAnthropic:
		return convertResponseFromAnthropic(to, resp)
	case ProtocolOpenAI:
		return convertResponseFromOpenAI(to, resp)
	case ProtocolGemini:
		return convertResponseFromGemini(to, resp)
	default:
		return nil, &ConversionError{From: from, To: to, Message: "unsupported source protocol"}
	}
}

// convertResponseFromAnthropic 从 Anthropic 响应转换
func convertResponseFromAnthropic(to ProtocolType, resp interface{}) (interface{}, error) {
	anthropicResp, ok := resp.(*protocol.AnthropicResponse)
	if !ok {
		return nil, &ConversionError{From: ProtocolAnthropic, To: to, Message: "invalid response type"}
	}

	switch to {
	case ProtocolOpenAI:
		return anthropicResponseToOpenAI(anthropicResp)
	case ProtocolGemini:
		return anthropicResponseToGemini(anthropicResp)
	case ProtocolAnthropic:
		return anthropicResp, nil
	default:
		return nil, &ConversionError{From: ProtocolAnthropic, To: to, Message: "unsupported target protocol"}
	}
}

// convertResponseFromOpenAI 从 OpenAI 响应转换
func convertResponseFromOpenAI(to ProtocolType, resp interface{}) (interface{}, error) {
	openaiResp, ok := resp.(*protocol.OpenAIResponse)
	if !ok {
		return nil, &ConversionError{From: ProtocolOpenAI, To: to, Message: "invalid response type"}
	}

	switch to {
	case ProtocolAnthropic:
		return openaiResponseToAnthropic(openaiResp)
	case ProtocolGemini:
		return openaiResponseToGemini(openaiResp)
	case ProtocolOpenAI:
		return openaiResp, nil
	default:
		return nil, &ConversionError{From: ProtocolOpenAI, To: to, Message: "unsupported target protocol"}
	}
}

// convertResponseFromGemini 从 Gemini 响应转换
func convertResponseFromGemini(to ProtocolType, resp interface{}) (interface{}, error) {
	geminiResp, ok := resp.(*protocol.GeminiResponse)
	if !ok {
		return nil, &ConversionError{From: ProtocolGemini, To: to, Message: "invalid response type"}
	}

	switch to {
	case ProtocolAnthropic:
		return geminiResponseToAnthropic(geminiResp)
	case ProtocolOpenAI:
		return geminiResponseToOpenAI(geminiResp)
	case ProtocolGemini:
		return geminiResp, nil
	default:
		return nil, &ConversionError{From: ProtocolGemini, To: to, Message: "unsupported target protocol"}
	}
}
