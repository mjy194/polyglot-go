package converter

import (
	"fmt"
	"polyglot/internal/protocol"
	"time"
)

// ============================================================================
// Anthropic → OpenAI Request Conversion
// ============================================================================

func anthropicToOpenAI(req *protocol.AnthropicRequest) (*protocol.OpenAIRequest, error) {
	mapper := NewModelMapper()

	// 转换消息
	var messages []protocol.OpenAIMessage

	// 添加 system 消息（如果有）
	if req.System != nil {
		if systemStr, ok := req.System.(string); ok && systemStr != "" {
			messages = append(messages, protocol.OpenAIMessage{
				Role:    "system",
				Content: systemStr,
			})
		}
	}

	// 转换用户/助手消息
	for _, msg := range req.Messages {
		openaiMsg := protocol.OpenAIMessage{
			Role: msg.Role,
		}

		// 转换 content
		if str, ok := msg.Content.(string); ok {
			openaiMsg.Content = str
		} else {
			// 处理 ContentBlock 数组（目前简化为字符串）
			openaiMsg.Content = protocol.ExtractUserMessage(msg)
		}

		messages = append(messages, openaiMsg)
	}

	// 转换工具
	var tools []protocol.Tool
	for _, tool := range req.Tools {
		tools = append(tools, protocol.Tool{
			Type: "function",
			Function: map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		})
	}

	return &protocol.OpenAIRequest{
		Model:       mapper.Map(ProtocolAnthropic, ProtocolOpenAI, req.Model),
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
		Tools:       tools,
	}, nil
}

// ============================================================================
// Anthropic → OpenAI Response Conversion
// ============================================================================

func anthropicResponseToOpenAI(resp *protocol.AnthropicResponse) (*protocol.OpenAIResponse, error) {
	mapper := NewModelMapper()

	// 提取内容和工具调用
	var content string
	var toolCalls []protocol.ToolCall

	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		} else if block.Type == "tool_use" {
			toolCalls = append(toolCalls, protocol.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: map[string]interface{}{
					"name":      block.Name,
					"arguments": fmt.Sprintf("%v", block.Input),
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
	if resp.StopReason == "max_tokens" {
		finishReason = "length"
	} else if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &protocol.OpenAIResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   mapper.Map(ProtocolAnthropic, ProtocolOpenAI, resp.Model),
		Choices: []protocol.Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: protocol.OpenAIUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// ============================================================================
// Anthropic → Gemini Request Conversion
// ============================================================================

func anthropicToGemini(req *protocol.AnthropicRequest) (*protocol.GeminiRequest, error) {
	// 转换消息
	var contents []protocol.GeminiContent

	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		var parts []protocol.GeminiPart

		// 转换 content
		if str, ok := msg.Content.(string); ok {
			parts = append(parts, protocol.GeminiPart{Text: str})
		} else {
			// 简化处理
			parts = append(parts, protocol.GeminiPart{
				Text: protocol.ExtractUserMessage(msg),
			})
		}

		contents = append(contents, protocol.GeminiContent{
			Role:  role,
			Parts: parts,
		})
	}

	// 转换工具
	var tools []protocol.GeminiTool
	if len(req.Tools) > 0 {
		var functionDeclarations []protocol.GeminiFunctionDeclaration
		for _, tool := range req.Tools {
			functionDeclarations = append(functionDeclarations, protocol.GeminiFunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
		tools = append(tools, protocol.GeminiTool{
			FunctionDeclarations: functionDeclarations,
		})
	}

	// 系统指令
	var systemInstruction *protocol.GeminiContent
	if req.System != nil {
		if systemStr, ok := req.System.(string); ok && systemStr != "" {
			systemInstruction = &protocol.GeminiContent{
				Parts: []protocol.GeminiPart{{Text: systemStr}},
			}
		}
	}

	return &protocol.GeminiRequest{
		Contents:          contents,
		Tools:             tools,
		SystemInstruction: systemInstruction,
		GenerationConfig: &protocol.GeminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			TopK:            req.TopK,
			MaxOutputTokens: req.MaxTokens,
			StopSequences:   req.StopSequences,
		},
	}, nil
}

// ============================================================================
// Anthropic → Gemini Response Conversion
// ============================================================================

func anthropicResponseToGemini(resp *protocol.AnthropicResponse) (*protocol.GeminiResponse, error) {
	// 转换内容
	var parts []protocol.GeminiPart

	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, protocol.GeminiPart{Text: block.Text})
		} else if block.Type == "tool_use" {
			parts = append(parts, protocol.GeminiPart{
				FunctionCall: &protocol.GeminiFunctionCall{
					Name: block.Name,
					Args: block.Input,
				},
			})
		}
	}

	// 转换 finish_reason
	finishReason := "STOP"
	if resp.StopReason == "max_tokens" {
		finishReason = "MAX_TOKENS"
	}

	return &protocol.GeminiResponse{
		Candidates: []protocol.GeminiCandidate{
			{
				Content: protocol.GeminiContent{
					Role:  "model",
					Parts: parts,
				},
				FinishReason: finishReason,
				Index:        0,
			},
		},
		UsageMetadata: &protocol.GeminiUsageMetadata{
			PromptTokenCount:     resp.Usage.InputTokens,
			CandidatesTokenCount: resp.Usage.OutputTokens,
			TotalTokenCount:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}
