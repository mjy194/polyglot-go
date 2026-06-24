package converter

import (
	"fmt"
	"polyglot/internal/protocol"
	"strings"
)

// ============================================================================
// Gemini → Anthropic Request Conversion
// ============================================================================

func geminiToAnthropic(req *protocol.GeminiRequest) (*protocol.AnthropicRequest, error) {
	mapper := NewModelMapper()

	// 转换消息
	var messages []protocol.AnthropicMessage

	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		// 提取文本内容
		var textParts []string
		for _, part := range content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}

		if len(textParts) > 0 {
			messages = append(messages, protocol.AnthropicMessage{
				Role:    role,
				Content: strings.Join(textParts, ""),
			})
		}
	}

	// 转换工具
	var tools []protocol.AnthropicTool
	for _, tool := range req.Tools {
		for _, funcDecl := range tool.FunctionDeclarations {
			tools = append(tools, protocol.AnthropicTool{
				Name:        funcDecl.Name,
				Description: funcDecl.Description,
				InputSchema: funcDecl.Parameters,
			})
		}
	}

	// 提取 system 指令
	var systemMsg string
	if req.SystemInstruction != nil {
		for _, part := range req.SystemInstruction.Parts {
			if part.Text != "" {
				systemMsg = part.Text
				break
			}
		}
	}

	// 获取生成配置
	maxTokens := 1024 // 默认值
	var temperature float64
	var topP float64
	var topK int
	var stopSequences []string

	if req.GenerationConfig != nil {
		if req.GenerationConfig.MaxOutputTokens > 0 {
			maxTokens = req.GenerationConfig.MaxOutputTokens
		}
		temperature = req.GenerationConfig.Temperature
		topP = req.GenerationConfig.TopP
		topK = req.GenerationConfig.TopK
		stopSequences = req.GenerationConfig.StopSequences
	}

	return &protocol.AnthropicRequest{
		Model:         mapper.Map(ProtocolGemini, ProtocolAnthropic, "gemini-pro"),
		Messages:      messages,
		MaxTokens:     maxTokens,
		System:        systemMsg,
		Temperature:   temperature,
		TopP:          topP,
		TopK:          topK,
		StopSequences: stopSequences,
		Tools:         tools,
	}, nil
}

// ============================================================================
// Gemini → Anthropic Response Conversion
// ============================================================================

func geminiResponseToAnthropic(resp *protocol.GeminiResponse) (*protocol.AnthropicResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, &ConversionError{
			From:    ProtocolGemini,
			To:      ProtocolAnthropic,
			Message: "no candidates in response",
		}
	}

	candidate := resp.Candidates[0]

	// 转换内容
	var content []protocol.ResponseContentBlock

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content = append(content, protocol.ResponseContentBlock{
				Type: "text",
				Text: part.Text,
			})
		} else if part.FunctionCall != nil {
			content = append(content, protocol.ResponseContentBlock{
				Type:  "tool_use",
				ID:    "toolu_" + fmt.Sprintf("%d", len(content)),
				Name:  part.FunctionCall.Name,
				Input: part.FunctionCall.Args,
			})
		}
	}

	// 转换 stop_reason
	stopReason := "end_turn"
	if candidate.FinishReason == "MAX_TOKENS" {
		stopReason = "max_tokens"
	}

	// 使用统计信息
	inputTokens := 0
	outputTokens := 0
	if resp.UsageMetadata != nil {
		inputTokens = resp.UsageMetadata.PromptTokenCount
		outputTokens = resp.UsageMetadata.CandidatesTokenCount
	}

	return &protocol.AnthropicResponse{
		ID:         "msg_gemini_" + fmt.Sprintf("%d", candidate.Index),
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      "claude-opus-4-8",
		StopReason: stopReason,
		Usage: protocol.Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}, nil
}

// ============================================================================
// Gemini → OpenAI Request Conversion
// ============================================================================

func geminiToOpenAI(req *protocol.GeminiRequest) (*protocol.OpenAIRequest, error) {
	mapper := NewModelMapper()

	// 转换消息
	var messages []protocol.OpenAIMessage

	// 添加 system 消息
	if req.SystemInstruction != nil {
		for _, part := range req.SystemInstruction.Parts {
			if part.Text != "" {
				messages = append(messages, protocol.OpenAIMessage{
					Role:    "system",
					Content: part.Text,
				})
				break
			}
		}
	}

	// 转换内容
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		// 提取文本内容
		var textParts []string
		for _, part := range content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
		}

		if len(textParts) > 0 {
			messages = append(messages, protocol.OpenAIMessage{
				Role:    role,
				Content: strings.Join(textParts, ""),
			})
		}
	}

	// 转换工具
	var tools []protocol.Tool
	for _, tool := range req.Tools {
		for _, funcDecl := range tool.FunctionDeclarations {
			tools = append(tools, protocol.Tool{
				Type: "function",
				Function: map[string]interface{}{
					"name":        funcDecl.Name,
					"description": funcDecl.Description,
					"parameters":  funcDecl.Parameters,
				},
			})
		}
	}

	// 获取生成配置
	maxTokens := 0
	var temperature float64
	var topP float64

	if req.GenerationConfig != nil {
		maxTokens = req.GenerationConfig.MaxOutputTokens
		temperature = req.GenerationConfig.Temperature
		topP = req.GenerationConfig.TopP
	}

	return &protocol.OpenAIRequest{
		Model:       mapper.Map(ProtocolGemini, ProtocolOpenAI, "gemini-pro"),
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		TopP:        topP,
		Tools:       tools,
	}, nil
}

// ============================================================================
// Gemini → OpenAI Response Conversion
// ============================================================================

func geminiResponseToOpenAI(resp *protocol.GeminiResponse) (*protocol.OpenAIResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, &ConversionError{
			From:    ProtocolGemini,
			To:      ProtocolOpenAI,
			Message: "no candidates in response",
		}
	}

	candidate := resp.Candidates[0]

	// 转换内容
	var content string
	var toolCalls []protocol.ToolCall

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			content += part.Text
		} else if part.FunctionCall != nil {
			toolCalls = append(toolCalls, protocol.ToolCall{
				ID:   "call_" + fmt.Sprintf("%d", len(toolCalls)),
				Type: "function",
				Function: map[string]interface{}{
					"name":      part.FunctionCall.Name,
					"arguments": fmt.Sprintf("%v", part.FunctionCall.Args),
				},
			})
		}
	}

	// 构建消息
	message := protocol.OpenAIMessage{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	// 转换 finish_reason
	finishReason := "stop"
	if candidate.FinishReason == "MAX_TOKENS" {
		finishReason = "length"
	} else if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	// 使用统计信息
	promptTokens := 0
	completionTokens := 0
	if resp.UsageMetadata != nil {
		promptTokens = resp.UsageMetadata.PromptTokenCount
		completionTokens = resp.UsageMetadata.CandidatesTokenCount
	}

	return &protocol.OpenAIResponse{
		ID:      "chatcmpl-gemini-" + fmt.Sprintf("%d", candidate.Index),
		Object:  "chat.completion",
		Created: 0, // 需要时间戳
		Model:   "gpt-4",
		Choices: []protocol.Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: protocol.OpenAIUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}, nil
}
