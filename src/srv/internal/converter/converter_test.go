package converter

import (
	"testing"

	"polyglot/internal/protocol"
)

// ============================================================================
// Anthropic → OpenAI Tests
// ============================================================================

func TestAnthropicToOpenAI(t *testing.T) {
	// 创建 Anthropic 请求
	anthropicReq := &protocol.AnthropicRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 100,
		System:    "You are a helpful assistant",
		Messages: []protocol.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Tools: []protocol.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}

	// 转换
	openaiReq, err := anthropicToOpenAI(anthropicReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证
	if openaiReq.Model != "gpt-4" {
		t.Errorf("Expected model gpt-4, got %s", openaiReq.Model)
	}

	if len(openaiReq.Messages) != 2 { // system + user
		t.Errorf("Expected 2 messages, got %d", len(openaiReq.Messages))
	}

	if openaiReq.Messages[0].Role != "system" {
		t.Errorf("First message should be system")
	}

	if len(openaiReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(openaiReq.Tools))
	}
}

// ============================================================================
// OpenAI → Anthropic Tests
// ============================================================================

func TestOpenAIToAnthropic(t *testing.T) {
	// 创建 OpenAI 请求
	openaiReq := &protocol.OpenAIRequest{
		Model:     "gpt-4",
		MaxTokens: 100,
		Messages: []protocol.OpenAIMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant",
			},
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    "done",
			},
		},
		Tools: []protocol.Tool{
			{
				Type: "function",
				Function: map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather information",
					"parameters": map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
	}

	// 转换
	anthropicReq, err := openaiToAnthropic(openaiReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证
	if anthropicReq.Model != "claude-opus-4-8" {
		t.Errorf("Expected model claude-opus-4-8, got %s", anthropicReq.Model)
	}

	if anthropicReq.System.(string) != "You are a helpful assistant" {
		t.Errorf("System message not converted correctly")
	}

	if len(anthropicReq.Messages) != 2 { // user + tool
		t.Errorf("Expected 2 messages, got %d", len(anthropicReq.Messages))
	}
	if anthropicReq.Messages[1].Role != "user" {
		t.Fatalf("tool result should become anthropic user content block, got role %q", anthropicReq.Messages[1].Role)
	}
	toolBlocks, ok := anthropicReq.Messages[1].Content.([]interface{})
	if !ok || len(toolBlocks) != 1 {
		t.Fatalf("tool message should become one tool_result block, got %#v", anthropicReq.Messages[1].Content)
	}
	block, ok := toolBlocks[0].(map[string]interface{})
	if !ok || block["type"] != "tool_result" || block["tool_use_id"] != "call_1" || block["content"] != "done" {
		t.Fatalf("unexpected tool_result block: %#v", toolBlocks[0])
	}

	if len(anthropicReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(anthropicReq.Tools))
	}
}

// ============================================================================
// Gemini → OpenAI Tests
// ============================================================================

func TestGeminiToOpenAI(t *testing.T) {
	// 创建 Gemini 请求
	geminiReq := &protocol.GeminiRequest{
		Contents: []protocol.GeminiContent{
			{
				Role: "user",
				Parts: []protocol.GeminiPart{
					{Text: "Hello, how are you?"},
					{FunctionResponse: &protocol.GeminiFunctionResponse{
						Name:     "get_weather",
						Response: map[string]interface{}{"ok": true},
					}},
				},
			},
		},
		SystemInstruction: &protocol.GeminiContent{
			Parts: []protocol.GeminiPart{
				{Text: "You are a helpful assistant"},
			},
		},
		Tools: []protocol.GeminiTool{
			{
				FunctionDeclarations: []protocol.GeminiFunctionDeclaration{
					{
						Name:        "get_weather",
						Description: "Get weather information",
						Parameters: map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
		},
	}

	// 转换
	openaiReq, err := geminiToOpenAI(geminiReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证
	if openaiReq.Model != "gpt-4" {
		t.Errorf("Expected model gpt-4, got %s", openaiReq.Model)
	}

	if len(openaiReq.Messages) != 2 { // system + user
		t.Errorf("Expected 2 messages, got %d", len(openaiReq.Messages))
	}
	if openaiReq.Messages[1].Role != "user" {
		t.Fatalf("expected second message role user, got %q", openaiReq.Messages[1].Role)
	}
	if content, ok := openaiReq.Messages[1].Content.(string); !ok || content != "Hello, how are you?" {
		t.Fatalf("expected text content preserved, got %#v", openaiReq.Messages[1].Content)
	}

	if len(openaiReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(openaiReq.Tools))
	}
}

// ============================================================================
// Model Mapping Tests
// ============================================================================

func TestModelMapper(t *testing.T) {
	mapper := NewModelMapper()

	tests := []struct {
		from     ProtocolType
		to       ProtocolType
		model    string
		expected string
	}{
		{ProtocolOpenAI, ProtocolAnthropic, "gpt-4", "claude-opus-4-8"},
		{ProtocolAnthropic, ProtocolOpenAI, "claude-opus-4-8", "gpt-4"},
		{ProtocolGemini, ProtocolOpenAI, "gemini-pro", "gpt-4"},
		{ProtocolOpenAI, ProtocolAnthropic, "unknown-model", "unknown-model"}, // 无映射时返回原值
	}

	for _, tt := range tests {
		result := mapper.Map(tt.from, tt.to, tt.model)
		if result != tt.expected {
			t.Errorf("Map(%s, %s, %s) = %s, expected %s",
				tt.from, tt.to, tt.model, result, tt.expected)
		}
	}
}
