package converter

import (
	"encoding/json"
	"fmt"
	"strings"

	"polyglot/internal/protocol"
	pb "polyglot/proto/adapter"

	"github.com/google/uuid"
)

// ============================================================================
// Anthropic → Universal
// ============================================================================

// AnthropicToUniversal 将 Anthropic 请求转换为统一格式
func AnthropicToUniversal(req *protocol.AnthropicRequest) (*pb.UniversalRequest, error) {
	// 转换消息
	var messages []*pb.Message
	for _, msg := range req.Messages {
		univMsg := &pb.Message{
			Role: convertAnthropicRole(msg.Role),
		}

		// 转换内容
		if str, ok := msg.Content.(string); ok {
			// 简单文本
			univMsg.Content = []*pb.ContentPart{
				{Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: str}}},
			}
		} else {
			// ContentBlock 数组
			univMsg.Content = convertAnthropicContent(msg.Content)
		}

		messages = append(messages, univMsg)
	}

	// 转换工具
	var tools []*pb.Tool
	for _, tool := range req.Tools {
		schemaJSON, _ := json.Marshal(tool.InputSchema)
		tools = append(tools, &pb.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  string(schemaJSON),
		})
	}

	system := anthropicSystemText(req.System)

	return &pb.UniversalRequest{
		RequestId: uuid.New().String(),
		Model:     req.Model,
		Messages:  messages,
		System:    system,
		Tools:     tools,
		Config: &pb.GenerationConfig{
			MaxTokens:     int32(req.MaxTokens),
			Temperature:   req.Temperature,
			TopP:          req.TopP,
			TopK:          int32(req.TopK),
			StopSequences: req.StopSequences,
			Stream:        req.Stream,
		},
	}, nil
}

func convertAnthropicRole(role string) pb.Message_Role {
	switch role {
	case "user":
		return pb.Message_USER
	case "assistant":
		return pb.Message_ASSISTANT
	case "system":
		return pb.Message_SYSTEM
	default:
		return pb.Message_ROLE_UNSPECIFIED
	}
}

func convertAnthropicContent(content interface{}) []*pb.ContentPart {
	var parts []*pb.ContentPart

	for _, block := range interfaceSlice(content) {
		if blockMap, ok := block.(map[string]interface{}); ok {
			blockType, _ := blockMap["type"].(string)

			switch blockType {
			case "text":
				if text, ok := blockMap["text"].(string); ok {
					parts = append(parts, textPartPB(text))
				}

			case "image":
				if source, ok := blockMap["source"].(map[string]interface{}); ok {
					imageType, _ := source["type"].(string)
					mimeType, _ := source["media_type"].(string)
					data, _ := source["data"].(string)
					url, _ := source["url"].(string)

					imagePart := &pb.ImagePart{MimeType: mimeType}
					if imageType == "base64" {
						imagePart.Source = &pb.ImagePart_Data{Data: []byte(data)}
					} else if imageType == "url" {
						if url == "" {
							url = data
						}
						imagePart.Source = &pb.ImagePart_Url{Url: url}
					}

					parts = append(parts, &pb.ContentPart{
						Part: &pb.ContentPart_Image{Image: imagePart},
					})
				}

			case "tool_use":
				if id, ok := blockMap["id"].(string); ok {
					if name, ok := blockMap["name"].(string); ok {
						inputJSON, _ := json.Marshal(blockMap["input"])
						parts = append(parts, &pb.ContentPart{
							Part: &pb.ContentPart_ToolCall{
								ToolCall: &pb.ToolCallPart{
									Id:        id,
									Name:      name,
									Arguments: string(inputJSON),
								},
							},
						})
					}
				}

			case "tool_result":
				if toolUseID, ok := blockMap["tool_use_id"].(string); ok {
					parts = append(parts, &pb.ContentPart{
						Part: &pb.ContentPart_ToolResult{
							ToolResult: &pb.ToolResultPart{
								ToolCallId: toolUseID,
								Result:     stringifyProtocolValue(blockMap["content"]),
								IsError:    boolFromAny(blockMap["is_error"]),
							},
						},
					})
				}
			}
		}
	}

	return parts
}

func anthropicSystemText(system interface{}) string {
	switch v := system.(type) {
	case nil:
		return ""
	case string:
		return v
	}
	var parts []string
	for _, block := range interfaceSlice(system) {
		if m, ok := block.(map[string]interface{}); ok {
			if text, ok := m["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func interfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if parts, ok := v.([]interface{}); ok {
		return parts
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var parts []interface{}
	if err := json.Unmarshal(b, &parts); err != nil {
		return nil
	}
	return parts
}

func boolFromAny(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func stringifyProtocolValue(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return fmt.Sprintf("%v", x)
		}
		return string(b)
	}
}

func textPartPB(text string) *pb.ContentPart {
	return &pb.ContentPart{
		Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: text}},
	}
}

func appendSystemText(system, text string) string {
	if strings.TrimSpace(text) == "" {
		return system
	}
	if strings.TrimSpace(system) == "" {
		return text
	}
	return system + "\n" + text
}

func textFromProtocolContent(content interface{}) string {
	if str, ok := content.(string); ok {
		return str
	}
	var parts []string
	for _, part := range interfaceSlice(content) {
		if m, ok := part.(map[string]interface{}); ok {
			if text, ok := m["text"].(string); ok && text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func stopSequencesFromAny(v interface{}) []string {
	switch x := v.(type) {
	case string:
		if x != "" {
			return []string{x}
		}
	case []string:
		return x
	case []interface{}:
		var out []string
		for _, raw := range x {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func metadataToStringMap(v map[string]interface{}) map[string]string {
	if len(v) == 0 {
		return nil
	}
	out := make(map[string]string, len(v))
	for key, value := range v {
		out[key] = stringifyProtocolValue(value)
	}
	return out
}

func maxOpenAITokens(req *protocol.OpenAIRequest) int {
	if req.MaxCompletionTokens > 0 {
		return req.MaxCompletionTokens
	}
	return req.MaxTokens
}

func toolResultID(toolCallID, name string) string {
	if toolCallID != "" {
		return toolCallID
	}
	return name
}

func appendOpenAIToolCall(parts []*pb.ContentPart, id string, function map[string]interface{}) []*pb.ContentPart {
	name := fmt.Sprintf("%v", function["name"])
	if id == "" {
		id = "call_" + name
	}
	argsJSON := stringifyToolCallArguments(function["arguments"])
	return append(parts, &pb.ContentPart{
		Part: &pb.ContentPart_ToolCall{
			ToolCall: &pb.ToolCallPart{
				Id:        id,
				Name:      name,
				Arguments: argsJSON,
			},
		},
	})
}

func appendTool(tools []*pb.Tool, name, description string, parameters interface{}) []*pb.Tool {
	if name == "" {
		return tools
	}
	paramsJSON, _ := json.Marshal(parameters)
	return append(tools, &pb.Tool{
		Name:        name,
		Description: description,
		Parameters:  string(paramsJSON),
	})
}

// ============================================================================
// Universal → Anthropic
// ============================================================================

// UniversalToAnthropic 将统一格式转换为 Anthropic 响应
func UniversalToAnthropic(requestID string, responses []*pb.UniversalResponse) (*protocol.AnthropicResponse, error) {
	var content []protocol.ResponseContentBlock
	var inputTokens, outputTokens int
	var finishReason string

	// 连续的文本 chunk 合并进同一个 text block：Anthropic 不会输出两个相邻的 text block。
	// 遇到工具调用等非文本响应时，把已缓冲的文本 flush 成一个 block。
	var textBuf strings.Builder
	flushText := func() {
		if textBuf.Len() > 0 {
			content = append(content, protocol.ResponseContentBlock{
				Type: "text",
				Text: textBuf.String(),
			})
			textBuf.Reset()
		}
	}

	for _, resp := range responses {
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			textBuf.WriteString(r.Chunk.Text)

		case *pb.UniversalResponse_ToolCall:
			// 工具调用前先 flush 已累积的文本
			flushText()
			content = append(content, protocol.ResponseContentBlock{
				Type:  "tool_use",
				ID:    r.ToolCall.Id,
				Name:  r.ToolCall.Name,
				Input: parseJSON(r.ToolCall.Arguments),
			})

		case *pb.UniversalResponse_Completion:
			// 完成信息
			inputTokens = int(r.Completion.InputTokens)
			outputTokens = int(r.Completion.OutputTokens)
			finishReason = r.Completion.FinishReason
		}
	}
	// 收尾：flush 最后一段文本
	flushText()

	return &protocol.AnthropicResponse{
		ID:         requestID,
		Type:       "message",
		Role:       "assistant",
		Content:    content,
		Model:      "claude-opus-4-8", // 可以从请求中获取
		StopReason: finishReason,
		Usage: protocol.Usage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	}, nil
}

// ============================================================================
// OpenAI → Universal
// ============================================================================

// OpenAIToUniversal 将 OpenAI 请求转换为统一格式
func OpenAIToUniversal(req *protocol.OpenAIRequest) (*pb.UniversalRequest, error) {
	var messages []*pb.Message
	var system string

	for _, msg := range req.Messages {
		if msg.Role == "system" || msg.Role == "developer" {
			system = appendSystemText(system, textFromProtocolContent(msg.Content))
			continue
		}

		univMsg := &pb.Message{
			Role: convertOpenAIRole(msg.Role),
		}

		if msg.Role == "tool" || msg.Role == "function" {
			univMsg.Role = pb.Message_TOOL
			univMsg.Content = []*pb.ContentPart{{
				Part: &pb.ContentPart_ToolResult{
					ToolResult: &pb.ToolResultPart{
						ToolCallId: toolResultID(msg.ToolCallID, msg.Name),
						Result:     stringifyProtocolValue(msg.Content),
					},
				},
			}}
			messages = append(messages, univMsg)
			continue
		}

		// 转换内容
		if str, ok := msg.Content.(string); ok {
			univMsg.Content = []*pb.ContentPart{
				{Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: str}}},
			}
		} else {
			univMsg.Content = convertOpenAIContent(msg.Content)
		}

		// 处理 tool_calls
		for _, toolCall := range msg.ToolCalls {
			univMsg.Content = appendOpenAIToolCall(univMsg.Content, toolCall.ID, toolCall.Function)
		}
		if msg.FunctionCall != nil {
			univMsg.Content = appendOpenAIToolCall(univMsg.Content, "", msg.FunctionCall)
		}

		messages = append(messages, univMsg)
	}

	// 转换工具
	var tools []*pb.Tool
	for _, tool := range req.Tools {
		if funcMap, ok := tool.Function["name"]; ok {
			tools = appendTool(tools, fmt.Sprintf("%v", funcMap), fmt.Sprintf("%v", tool.Function["description"]), tool.Function["parameters"])
		}
	}
	for _, fn := range req.Functions {
		tools = appendTool(tools, fn.Name, fn.Description, fn.Parameters)
	}

	context := &pb.RequestContext{
		UserId:   req.User,
		Metadata: metadataToStringMap(req.Metadata),
	}
	if context.UserId == "" && len(context.Metadata) == 0 {
		context = nil
	}

	return &pb.UniversalRequest{
		RequestId: uuid.New().String(),
		Model:     req.Model,
		Messages:  messages,
		System:    system,
		Tools:     tools,
		Config: &pb.GenerationConfig{
			MaxTokens:     int32(maxOpenAITokens(req)),
			Temperature:   req.Temperature,
			TopP:          req.TopP,
			StopSequences: stopSequencesFromAny(req.Stop),
			Stream:        req.Stream,
		},
		Context: context,
	}, nil
}

func convertOpenAIRole(role string) pb.Message_Role {
	switch role {
	case "user":
		return pb.Message_USER
	case "assistant":
		return pb.Message_ASSISTANT
	case "system":
		return pb.Message_SYSTEM
	case "tool", "function":
		return pb.Message_TOOL
	default:
		return pb.Message_ROLE_UNSPECIFIED
	}
}

func convertOpenAIContent(content interface{}) []*pb.ContentPart {
	var parts []*pb.ContentPart

	for _, part := range interfaceSlice(content) {
		if partMap, ok := part.(map[string]interface{}); ok {
			partType, _ := partMap["type"].(string)

			switch partType {
			case "text", "input_text":
				if text, ok := partMap["text"].(string); ok {
					parts = append(parts, textPartPB(text))
				}

			case "image_url", "input_image":
				url, detail := openAIImageURL(partMap)
				if url != "" {
					parts = append(parts, &pb.ContentPart{
						Part: &pb.ContentPart_Image{
							Image: &pb.ImagePart{
								Source: &pb.ImagePart_Url{Url: url},
								Detail: detail,
							},
						},
					})
				}
			}
		}
	}

	return parts
}

func openAIImageURL(partMap map[string]interface{}) (url string, detail string) {
	if d, ok := partMap["detail"].(string); ok {
		detail = d
	}
	switch raw := partMap["image_url"].(type) {
	case string:
		url = raw
	case map[string]interface{}:
		if s, ok := raw["url"].(string); ok {
			url = s
		}
		if d, ok := raw["detail"].(string); ok {
			detail = d
		}
	}
	if url == "" {
		if s, ok := partMap["image"].(string); ok {
			url = s
		}
	}
	return url, detail
}

// ============================================================================
// Universal → OpenAI
// ============================================================================

// UniversalToOpenAI 将统一格式转换为 OpenAI 响应
func UniversalToOpenAI(requestID string, model string, responses []*pb.UniversalResponse) (*protocol.OpenAIResponse, error) {
	var content string
	var toolCalls []protocol.ToolCall
	var inputTokens, outputTokens int
	var finishReason string

	for _, resp := range responses {
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			content += r.Chunk.Text

		case *pb.UniversalResponse_ToolCall:
			toolCalls = append(toolCalls, protocol.ToolCall{
				ID:   r.ToolCall.Id,
				Type: "function",
				Function: map[string]interface{}{
					"name":      r.ToolCall.Name,
					"arguments": r.ToolCall.Arguments,
				},
			})

		case *pb.UniversalResponse_Completion:
			inputTokens = int(r.Completion.InputTokens)
			outputTokens = int(r.Completion.OutputTokens)
			finishReason = r.Completion.FinishReason
		}
	}

	message := protocol.OpenAIMessage{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
		finishReason = "tool_calls"
	}

	return &protocol.OpenAIResponse{
		ID:      requestID,
		Object:  "chat.completion",
		Created: 0, // 需要时间戳
		Model:   model,
		Choices: []protocol.Choice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: protocol.OpenAIUsage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
	}, nil
}

// ============================================================================
// Gemini → Universal
// ============================================================================

// GeminiToUniversal 将 Gemini 请求转换为统一格式
func GeminiToUniversal(req *protocol.GeminiRequest) (*pb.UniversalRequest, error) {
	var messages []*pb.Message
	var system string

	// 提取 system
	if req.SystemInstruction != nil {
		for _, part := range req.SystemInstruction.Parts {
			system += part.Text
		}
	}

	// 转换消息
	for _, content := range req.Contents {
		univMsg := &pb.Message{
			Role: convertGeminiRole(content.Role),
		}

		// 转换内容
		for _, part := range content.Parts {
			if part.Text != "" {
				univMsg.Content = append(univMsg.Content, &pb.ContentPart{
					Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: part.Text}},
				})
			}

			if part.InlineData != nil {
				univMsg.Content = append(univMsg.Content, &pb.ContentPart{
					Part: &pb.ContentPart_Image{
						Image: &pb.ImagePart{
							Source:   &pb.ImagePart_Data{Data: []byte(part.InlineData.Data)},
							MimeType: part.InlineData.MimeType,
						},
					},
				})
			}

			if part.FileData != nil {
				univMsg.Content = append(univMsg.Content, &pb.ContentPart{
					Part: &pb.ContentPart_Image{
						Image: &pb.ImagePart{
							Source:   &pb.ImagePart_Url{Url: part.FileData.FileURI},
							MimeType: part.FileData.MimeType,
						},
					},
				})
			}

			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				univMsg.Content = append(univMsg.Content, &pb.ContentPart{
					Part: &pb.ContentPart_ToolCall{
						ToolCall: &pb.ToolCallPart{
							Id:        "gemini_" + part.FunctionCall.Name,
							Name:      part.FunctionCall.Name,
							Arguments: string(argsJSON),
						},
					},
				})
			}

			if part.FunctionResponse != nil {
				resultJSON, _ := json.Marshal(part.FunctionResponse.Response)
				univMsg.Content = append(univMsg.Content, &pb.ContentPart{
					Part: &pb.ContentPart_ToolResult{
						ToolResult: &pb.ToolResultPart{
							ToolCallId: "gemini_" + part.FunctionResponse.Name,
							Result:     string(resultJSON),
						},
					},
				})
			}
		}

		messages = append(messages, univMsg)
	}

	// 转换工具
	var tools []*pb.Tool
	for _, tool := range req.Tools {
		for _, funcDecl := range tool.FunctionDeclarations {
			paramsJSON, _ := json.Marshal(funcDecl.Parameters)
			tools = append(tools, &pb.Tool{
				Name:        funcDecl.Name,
				Description: funcDecl.Description,
				Parameters:  string(paramsJSON),
			})
		}
	}

	config := &pb.GenerationConfig{}
	if req.GenerationConfig != nil {
		config.MaxTokens = int32(req.GenerationConfig.MaxOutputTokens)
		config.Temperature = req.GenerationConfig.Temperature
		config.TopP = req.GenerationConfig.TopP
		config.TopK = int32(req.GenerationConfig.TopK)
		config.StopSequences = req.GenerationConfig.StopSequences
	}

	model := req.Model
	if model == "" {
		model = "gemini-pro"
	}

	return &pb.UniversalRequest{
		RequestId: uuid.New().String(),
		Model:     model,
		Messages:  messages,
		System:    system,
		Tools:     tools,
		Config:    config,
	}, nil
}

func convertGeminiRole(role string) pb.Message_Role {
	switch role {
	case "user":
		return pb.Message_USER
	case "model":
		return pb.Message_ASSISTANT
	default:
		return pb.Message_ROLE_UNSPECIFIED
	}
}

// ============================================================================
// Universal → Gemini
// ============================================================================

// UniversalToGemini 将统一格式转换为 Gemini 响应
func UniversalToGemini(responses []*pb.UniversalResponse) (*protocol.GeminiResponse, error) {
	var parts []protocol.GeminiPart
	var inputTokens, outputTokens int
	var finishReason string

	// 连续文本 chunk 合并进单个 part（Gemini 客户端期望合并文本，而非每个 delta 一个 part）
	var textBuf strings.Builder
	flushText := func() {
		if textBuf.Len() > 0 {
			parts = append(parts, protocol.GeminiPart{Text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for _, resp := range responses {
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			textBuf.WriteString(r.Chunk.Text)

		case *pb.UniversalResponse_ToolCall:
			flushText()
			parts = append(parts, protocol.GeminiPart{
				FunctionCall: &protocol.GeminiFunctionCall{
					Name: r.ToolCall.Name,
					Args: parseJSON(r.ToolCall.Arguments),
				},
			})

		case *pb.UniversalResponse_Completion:
			inputTokens = int(r.Completion.InputTokens)
			outputTokens = int(r.Completion.OutputTokens)
			finishReason = r.Completion.FinishReason
		}
	}
	flushText()

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
			PromptTokenCount:     inputTokens,
			CandidatesTokenCount: outputTokens,
			TotalTokenCount:      inputTokens + outputTokens,
		},
	}, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func parseJSON(jsonStr string) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &result)
	return result
}

func stringifyToolCallArguments(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "{}"
	default:
		b, err := json.Marshal(x)
		if err != nil {
			return "{}"
		}
		return string(b)
	}
}
