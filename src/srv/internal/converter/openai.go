package converter

import (
	"encoding/json"
	"fmt"
	"polyglot/internal/protocol"
	"strings"
)

// ============================================================================
// OpenAI → Anthropic Request Conversion
// ============================================================================

func openaiToAnthropic(req *protocol.OpenAIRequest) (*protocol.AnthropicRequest, error) {
	mapper := NewModelMapper()

	// 提取 system 消息
	var systemMsg string
	var messages []protocol.AnthropicMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// 提取 system 内容
			if content, ok := msg.Content.(string); ok {
				systemMsg = content
			}
		} else {
			anthropicMsg := protocol.AnthropicMessage{Role: openaiRoleToAnthropic(msg.Role)}
			anthropicMsg.Content = openAIMessageToAnthropicContent(msg)
			messages = append(messages, anthropicMsg)
		}
	}

	// 转换工具
	var tools []protocol.AnthropicTool
	for _, tool := range req.Tools {
		if functionMap, ok := tool.Function["name"]; ok {
			tools = append(tools, protocol.AnthropicTool{
				Name:        fmt.Sprintf("%v", functionMap),
				Description: fmt.Sprintf("%v", tool.Function["description"]),
				InputSchema: tool.Function["parameters"].(map[string]interface{}),
			})
		}
	}

	return &protocol.AnthropicRequest{
		Model:       mapper.Map(ProtocolOpenAI, ProtocolAnthropic, req.Model),
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		System:      systemMsg,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Tools:       tools,
	}, nil
}

func stringifyOpenAIContent(content interface{}) string {
	if str, ok := content.(string); ok {
		return str
	}
	b, err := json.Marshal(content)
	if err != nil {
		return fmt.Sprintf("%v", content)
	}
	return string(b)
}

func openAIContentToAnthropicBlocks(content interface{}) interface{} {
	contentParts, ok := content.([]interface{})
	if !ok {
		return fmt.Sprintf("%v", content)
	}

	blocks := make([]interface{}, 0, len(contentParts))
	for _, rawPart := range contentParts {
		partMap, ok := rawPart.(map[string]interface{})
		if !ok {
			continue
		}

		switch partMap["type"] {
		case "text", "input_text", "output_text":
			if text, ok := partMap["text"].(string); ok {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": text,
				})
			}
		case "image_url":
			if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
				if url, ok := imageURL["url"].(string); ok {
					blocks = append(blocks, map[string]interface{}{
						"type": "image",
						"source": map[string]interface{}{
							"type": "url",
							"data": url,
						},
					})
				}
			}
		}
	}

	if len(blocks) == 0 {
		return fmt.Sprintf("%v", content)
	}
	return blocks
}

func openAIMessageToAnthropicContent(msg protocol.OpenAIMessage) interface{} {
	if msg.Role == "tool" {
		return []interface{}{
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     stringifyOpenAIContent(msg.Content),
			},
		}
	}

	var blocks []interface{}
	if str, ok := msg.Content.(string); ok {
		if str != "" {
			blocks = append(blocks, map[string]interface{}{
				"type": "text",
				"text": str,
			})
		}
	} else if msg.Content != nil {
		if converted, ok := openAIContentToAnthropicBlocks(msg.Content).([]interface{}); ok {
			blocks = append(blocks, converted...)
		}
	}

	for _, toolCall := range msg.ToolCalls {
		blocks = append(blocks, map[string]interface{}{
			"type":  "tool_use",
			"id":    toolCall.ID,
			"name":  fmt.Sprintf("%v", toolCall.Function["name"]),
			"input": parseOpenAIToolArguments(toolCall.Function["arguments"]),
		})
	}

	if len(blocks) == 0 {
		return ""
	}
	if len(blocks) == 1 {
		if blk, ok := blocks[0].(map[string]interface{}); ok && blk["type"] == "text" {
			if text, ok := blk["text"].(string); ok {
				return text
			}
		}
	}
	return blocks
}

func openaiRoleToAnthropic(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	case "tool":
		return "user"
	default:
		return "user"
	}
}

func parseOpenAIToolArguments(v interface{}) interface{} {
	switch x := v.(type) {
	case string:
		if x == "" {
			return map[string]interface{}{}
		}
		var out interface{}
		if err := json.Unmarshal([]byte(x), &out); err == nil {
			return out
		}
		return x
	case map[string]interface{}:
		return x
	case nil:
		return map[string]interface{}{}
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	}
}

// ============================================================================
// OpenAI → Anthropic Response Conversion
// ============================================================================

func openaiResponseToAnthropic(resp *protocol.OpenAIResponse) (*protocol.AnthropicResponse, error) {
	mapper := NewModelMapper()

	if len(resp.Choices) == 0 {
		return nil, &ConversionError{
			From:    ProtocolOpenAI,
			To:      ProtocolAnthropic,
			Message: "no choices in response",
		}
	}

	choice := resp.Choices[0]

	// 转换内容
	var content []protocol.ResponseContentBlock

	// 添加文本内容
	if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
		content = append(content, protocol.ResponseContentBlock{
			Type: "text",
			Text: contentStr,
		})
	}

	// 添加工具调用
	for _, toolCall := range choice.Message.ToolCalls {
		var input map[string]interface{}
		if argsStr, ok := toolCall.Function["arguments"].(string); ok {
			json.Unmarshal([]byte(argsStr), &input)
		}

		content = append(content, protocol.ResponseContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  fmt.Sprintf("%v", toolCall.Function["name"]),
			Input: input,
		})
	}

	// 转换 stop_reason
	stopReason := "end_turn"
	if choice.FinishReason == "length" {
		stopReason = "max_tokens"
	}

	return &protocol.AnthropicResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      mapper.Map(ProtocolOpenAI, ProtocolAnthropic, resp.Model),
		StopReason: stopReason,
		Usage: protocol.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}, nil
}

// ============================================================================
// OpenAI → Gemini Request Conversion
// ============================================================================

func openaiToGemini(req *protocol.OpenAIRequest) (*protocol.GeminiRequest, error) {
	// 提取 system 消息
	var systemInstruction *protocol.GeminiContent
	var contents []protocol.GeminiContent

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// 提取 system 内容
			if content, ok := msg.Content.(string); ok {
				systemInstruction = &protocol.GeminiContent{
					Parts: []protocol.GeminiPart{{Text: content}},
				}
			}
		} else {
			// 转换 role
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}

			// 转换 content
			var parts []protocol.GeminiPart
			if str, ok := msg.Content.(string); ok {
				parts = append(parts, protocol.GeminiPart{Text: str})
			} else {
				parts = append(parts, protocol.GeminiPart{
					Text: fmt.Sprintf("%v", msg.Content),
				})
			}

			contents = append(contents, protocol.GeminiContent{
				Role:  role,
				Parts: parts,
			})
		}
	}

	// 转换工具
	var tools []protocol.GeminiTool
	if len(req.Tools) > 0 {
		var functionDeclarations []protocol.GeminiFunctionDeclaration
		for _, tool := range req.Tools {
			if functionMap, ok := tool.Function["name"]; ok {
				functionDeclarations = append(functionDeclarations, protocol.GeminiFunctionDeclaration{
					Name:        fmt.Sprintf("%v", functionMap),
					Description: fmt.Sprintf("%v", tool.Function["description"]),
					Parameters:  tool.Function["parameters"].(map[string]interface{}),
				})
			}
		}
		tools = append(tools, protocol.GeminiTool{
			FunctionDeclarations: functionDeclarations,
		})
	}

	return &protocol.GeminiRequest{
		Contents:          contents,
		Tools:             tools,
		SystemInstruction: systemInstruction,
		GenerationConfig: &protocol.GeminiGenerationConfig{
			Temperature:     req.Temperature,
			TopP:            req.TopP,
			MaxOutputTokens: req.MaxTokens,
		},
	}, nil
}

// ============================================================================
// OpenAI → Gemini Response Conversion
// ============================================================================

func openaiResponseToGemini(resp *protocol.OpenAIResponse) (*protocol.GeminiResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, &ConversionError{
			From:    ProtocolOpenAI,
			To:      ProtocolGemini,
			Message: "no choices in response",
		}
	}

	choice := resp.Choices[0]

	// 转换内容
	var parts []protocol.GeminiPart

	// 添加文本内容
	if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
		parts = append(parts, protocol.GeminiPart{Text: contentStr})
	}

	// 添加工具调用
	for _, toolCall := range choice.Message.ToolCalls {
		var args map[string]interface{}
		if argsStr, ok := toolCall.Function["arguments"].(string); ok {
			json.Unmarshal([]byte(argsStr), &args)
		}

		parts = append(parts, protocol.GeminiPart{
			FunctionCall: &protocol.GeminiFunctionCall{
				Name: fmt.Sprintf("%v", toolCall.Function["name"]),
				Args: args,
			},
		})
	}

	// 转换 finish_reason
	finishReason := "STOP"
	if choice.FinishReason == "length" {
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
			PromptTokenCount:     resp.Usage.PromptTokens,
			CandidatesTokenCount: resp.Usage.CompletionTokens,
			TotalTokenCount:      resp.Usage.TotalTokens,
		},
	}, nil
}
