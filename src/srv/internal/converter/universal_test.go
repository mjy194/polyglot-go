package converter

import (
	"encoding/json"
	"testing"

	"polyglot/internal/protocol"
	pb "polyglot/proto/adapter"
)

// ============================================================================
// Anthropic → Universal Tests
// ============================================================================

func TestAnthropicToUniversal(t *testing.T) {
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
		Temperature: 0.7,
		TopP:        0.9,
		TopK:        40,
		Stream:      true,
	}

	univReq, err := AnthropicToUniversal(anthropicReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证基本字段
	if univReq.Model != "claude-opus-4-8" {
		t.Errorf("Expected model claude-opus-4-8, got %s", univReq.Model)
	}

	if univReq.System != "You are a helpful assistant" {
		t.Errorf("System not converted correctly")
	}

	// 验证消息
	if len(univReq.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(univReq.Messages))
	}

	if univReq.Messages[0].Role != pb.Message_USER {
		t.Errorf("Expected USER role")
	}

	// 验证内容
	if len(univReq.Messages[0].Content) != 1 {
		t.Errorf("Expected 1 content part")
	}

	textPart := univReq.Messages[0].Content[0].GetText()
	if textPart == nil {
		t.Errorf("Expected text part")
	} else if textPart.Text != "Hello, how are you?" {
		t.Errorf("Text content not correct")
	}

	// 验证工具
	if len(univReq.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(univReq.Tools))
	}

	if univReq.Tools[0].Name != "get_weather" {
		t.Errorf("Tool name not correct")
	}

	// 验证配置
	if univReq.Config.MaxTokens != 100 {
		t.Errorf("MaxTokens not correct")
	}

	if univReq.Config.Temperature != 0.7 {
		t.Errorf("Temperature not correct")
	}

	if !univReq.Config.Stream {
		t.Errorf("Stream should be true")
	}
}

// ============================================================================
// OpenAI → Universal Tests
// ============================================================================

func TestOpenAIToUniversal(t *testing.T) {
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
				Content: "Hello",
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    "42",
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
		Temperature: 0.7,
		Stream:      true,
	}

	univReq, err := OpenAIToUniversal(openaiReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证 system 提取
	if univReq.System != "You are a helpful assistant" {
		t.Errorf("System not extracted correctly")
	}

	// 验证消息（不包含 system）
	if len(univReq.Messages) != 2 {
		t.Errorf("Expected 2 messages (excluding system), got %d", len(univReq.Messages))
	}
	if got := univReq.Messages[1].GetContent()[0].GetToolResult(); got == nil || got.ToolCallId != "call_1" || got.Result != "42" {
		t.Fatalf("tool role should become tool_result, got %+v", univReq.Messages[1].GetContent())
	}

	// 验证工具
	if len(univReq.Tools) != 1 {
		t.Errorf("Expected 1 tool")
	}
}

// ============================================================================
// Gemini → Universal Tests
// ============================================================================

func TestGeminiToUniversal(t *testing.T) {
	geminiReq := &protocol.GeminiRequest{
		Contents: []protocol.GeminiContent{
			{
				Role: "user",
				Parts: []protocol.GeminiPart{
					{Text: "Hello"},
					{FunctionResponse: &protocol.GeminiFunctionResponse{
						Name:     "lookup",
						Response: map[string]interface{}{"value": "ok"},
					}},
				},
			},
		},
		SystemInstruction: &protocol.GeminiContent{
			Parts: []protocol.GeminiPart{
				{Text: "You are helpful"},
			},
		},
		Tools: []protocol.GeminiTool{
			{
				FunctionDeclarations: []protocol.GeminiFunctionDeclaration{
					{
						Name:        "get_weather",
						Description: "Get weather",
						Parameters: map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
		},
		GenerationConfig: &protocol.GeminiGenerationConfig{
			Temperature:     0.7,
			TopP:            0.9,
			TopK:            40,
			MaxOutputTokens: 100,
		},
	}

	univReq, err := GeminiToUniversal(geminiReq)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证 system
	if univReq.System != "You are helpful" {
		t.Errorf("System not extracted correctly")
	}

	// 验证消息
	if len(univReq.Messages) != 1 {
		t.Errorf("Expected 1 message")
	}
	if got := univReq.Messages[0].GetContent()[1].GetToolResult(); got == nil || got.ToolCallId != "gemini_lookup" {
		t.Fatalf("functionResponse should become tool_result, got %+v", univReq.Messages[0].GetContent())
	}

	// 验证工具
	if len(univReq.Tools) != 1 {
		t.Errorf("Expected 1 tool")
	}
}

func TestAnthropicToUniversalSystemBlocksImageURLAndToolResultBlocks(t *testing.T) {
	req := &protocol.AnthropicRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 100,
		System: []interface{}{
			map[string]interface{}{"type": "text", "text": "sys one"},
			map[string]interface{}{"type": "text", "text": "sys two"},
		},
		Messages: []protocol.AnthropicMessage{{
			Role: "user",
			Content: []interface{}{
				map[string]interface{}{"type": "image", "source": map[string]interface{}{"type": "url", "url": "https://example.test/a.png"}},
				map[string]interface{}{"type": "tool_result", "tool_use_id": "call_1", "content": []interface{}{map[string]interface{}{"type": "text", "text": "done"}}, "is_error": true},
			},
		}},
	}

	univReq, err := AnthropicToUniversal(req)
	if err != nil {
		t.Fatalf("AnthropicToUniversal failed: %v", err)
	}
	if univReq.System != "sys one\nsys two" {
		t.Fatalf("system = %q, want joined system blocks", univReq.System)
	}
	img := univReq.Messages[0].GetContent()[0].GetImage()
	if img == nil || img.GetUrl() != "https://example.test/a.png" {
		t.Fatalf("image URL not preserved: %+v", univReq.Messages[0].GetContent()[0])
	}
	tr := univReq.Messages[0].GetContent()[1].GetToolResult()
	if tr == nil || tr.ToolCallId != "call_1" || !tr.IsError || tr.Result != `[{"text":"done","type":"text"}]` {
		t.Fatalf("tool_result blocks not preserved: %+v", tr)
	}
}

func TestOpenAIToUniversalCompletesModernAndLegacyChatShapes(t *testing.T) {
	req := &protocol.OpenAIRequest{
		Model:               "gpt-4.1",
		MaxTokens:           50,
		MaxCompletionTokens: 123,
		Stop:                []interface{}{"END"},
		User:                "user_1",
		Metadata:            map[string]interface{}{"trace": "abc"},
		Messages: []protocol.OpenAIMessage{
			{Role: "developer", Content: "developer rules"},
			{Role: "user", Content: []interface{}{
				map[string]interface{}{"type": "text", "text": "look"},
				map[string]interface{}{"type": "image_url", "image_url": "https://example.test/img.png"},
			}},
			{Role: "assistant", FunctionCall: map[string]interface{}{"name": "lookup", "arguments": map[string]interface{}{"city": "SF"}}},
			{Role: "function", Name: "lookup", Content: map[string]interface{}{"ok": true}},
		},
		Functions: []protocol.OpenAIFunction{{
			Name:        "lookup",
			Description: "Lookup data",
			Parameters:  map[string]interface{}{"type": "object"},
		}},
	}

	univReq, err := OpenAIToUniversal(req)
	if err != nil {
		t.Fatalf("OpenAIToUniversal failed: %v", err)
	}
	if univReq.System != "developer rules" {
		t.Fatalf("system = %q, want developer rules", univReq.System)
	}
	if univReq.Config.MaxTokens != 123 || len(univReq.Config.StopSequences) != 1 || univReq.Config.StopSequences[0] != "END" {
		t.Fatalf("config not mapped: %+v", univReq.Config)
	}
	if univReq.Context == nil || univReq.Context.UserId != "user_1" || univReq.Context.Metadata["trace"] != "abc" {
		t.Fatalf("context not mapped: %+v", univReq.Context)
	}
	if len(univReq.Tools) != 1 || univReq.Tools[0].Name != "lookup" {
		t.Fatalf("legacy function not mapped as tool: %+v", univReq.Tools)
	}
	img := univReq.Messages[0].GetContent()[1].GetImage()
	if img == nil || img.GetUrl() != "https://example.test/img.png" {
		t.Fatalf("image_url string not preserved: %+v", univReq.Messages[0].GetContent())
	}
	tc := univReq.Messages[1].GetContent()[0].GetToolCall()
	if tc == nil || tc.Id != "call_lookup" || tc.Arguments != `{"city":"SF"}` {
		t.Fatalf("legacy function_call not mapped: %+v", tc)
	}
	tr := univReq.Messages[2].GetContent()[0].GetToolResult()
	if tr == nil || tr.ToolCallId != "lookup" || tr.Result != `{"ok":true}` {
		t.Fatalf("legacy function result not mapped: %+v", tr)
	}
}

func TestResponsesToUniversalImageAndStructuredFunctionOutput(t *testing.T) {
	req := &protocol.ResponseRequest{
		Model:        "gpt-4.1",
		Instructions: []byte(`"base instructions"`),
		Input: []byte(`[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"dev rules"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"look"},{"type":"input_image","image_url":{"url":"https://example.test/r.png","detail":"high"}}]},
			{"type":"function_call_output","call_id":"call_1","output":{"ok":true}}
		]`),
	}
	univReq, err := ResponsesToUniversal(req)
	if err != nil {
		t.Fatalf("ResponsesToUniversal failed: %v", err)
	}
	if univReq.System != "base instructions\ndev rules" {
		t.Fatalf("system = %q", univReq.System)
	}
	if len(univReq.Messages) != 2 {
		t.Fatalf("messages = %d, want user + tool", len(univReq.Messages))
	}
	img := univReq.Messages[0].GetContent()[1].GetImage()
	if img == nil || img.GetUrl() != "https://example.test/r.png" || img.Detail != "high" {
		t.Fatalf("Responses input_image not mapped: %+v", img)
	}
	tr := univReq.Messages[1].GetContent()[0].GetToolResult()
	if tr == nil || tr.ToolCallId != "call_1" || tr.Result != `{"ok":true}` {
		t.Fatalf("structured output not preserved: %+v", tr)
	}
}

func TestGeminiSnakeCaseRequestAndFileDataToUniversal(t *testing.T) {
	var req protocol.GeminiRequest
	if err := json.Unmarshal([]byte(`{
		"system_instruction":{"parts":[{"text":"sys"}]},
		"generation_config":{"max_output_tokens":77,"top_p":0.8,"top_k":32,"stop_sequences":["END"],"response_mime_type":"application/json","response_json_schema":{"type":"object"}},
		"tools":[{"function_declarations":[{"name":"lookup","description":"Lookup","parameters":{"type":"object"}}]}],
		"contents":[{"role":"user","parts":[{"text":"hi"},{"file_data":{"mime_type":"image/png","file_uri":"gs://bucket/img.png"}}]}]
	}`), &req); err != nil {
		t.Fatalf("unmarshal Gemini snake_case request failed: %v", err)
	}
	if req.GenerationConfig == nil || req.GenerationConfig.MaxOutputTokens != 77 || req.GenerationConfig.ResponseMimeType != "application/json" || req.GenerationConfig.ResponseJsonSchema["type"] != "object" {
		t.Fatalf("generation_config not decoded: %+v", req.GenerationConfig)
	}
	if len(req.Tools) != 1 || len(req.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("function_declarations not decoded: %+v", req.Tools)
	}

	univReq, err := GeminiToUniversal(&req)
	if err != nil {
		t.Fatalf("GeminiToUniversal failed: %v", err)
	}
	if univReq.System != "sys" || univReq.Config.MaxTokens != 77 || univReq.Config.TopP != 0.8 || univReq.Config.TopK != 32 {
		t.Fatalf("Gemini config/system not mapped: system=%q config=%+v", univReq.System, univReq.Config)
	}
	img := univReq.Messages[0].GetContent()[1].GetImage()
	if img == nil || img.GetUrl() != "gs://bucket/img.png" || img.MimeType != "image/png" {
		t.Fatalf("file_data not mapped: %+v", img)
	}
}

// ============================================================================
// Universal → Protocol Tests
// ============================================================================

func TestUniversalToAnthropic(t *testing.T) {
	responses := []*pb.UniversalResponse{
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Chunk{
				Chunk: &pb.ContentChunk{
					Text:  "Hello",
					Index: 0,
				},
			},
		},
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Chunk{
				Chunk: &pb.ContentChunk{
					Text:    " world",
					Index:   1,
					IsFinal: true,
				},
			},
		},
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Completion{
				Completion: &pb.CompletionInfo{
					FinishReason: "stop",
					InputTokens:  10,
					OutputTokens: 20,
				},
			},
		},
	}

	anthropicResp, err := UniversalToAnthropic("req_123", responses)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证内容：连续文本 chunk 应合并为单个 text block（Anthropic 不输出相邻 text block）
	if len(anthropicResp.Content) != 1 {
		t.Fatalf("Expected 1 merged text block, got %d: %+v", len(anthropicResp.Content), anthropicResp.Content)
	}
	if anthropicResp.Content[0].Type != "text" || anthropicResp.Content[0].Text != "Hello world" {
		t.Errorf("Expected merged text 'Hello world', got %+v", anthropicResp.Content[0])
	}

	// 验证 token 统计
	if anthropicResp.Usage.InputTokens != 10 {
		t.Errorf("InputTokens not correct")
	}

	if anthropicResp.Usage.OutputTokens != 20 {
		t.Errorf("OutputTokens not correct")
	}
}

func TestUniversalToAnthropicMergesTextAroundToolUse(t *testing.T) {
	// text "Hello" + text " world" → 合并成 1 个 text block；
	// 随后 tool_use，再出现 text "done" → 应是第二个独立的 text block。
	responses := []*pb.UniversalResponse{
		{Response: &pb.UniversalResponse_Chunk{Chunk: &pb.ContentChunk{Text: "Hello"}}},
		{Response: &pb.UniversalResponse_Chunk{Chunk: &pb.ContentChunk{Text: " world"}}},
		{Response: &pb.UniversalResponse_ToolCall{ToolCall: &pb.ToolCall{Id: "t1", Name: "f", Arguments: `{}`}}},
		{Response: &pb.UniversalResponse_Chunk{Chunk: &pb.ContentChunk{Text: "done"}}},
		{Response: &pb.UniversalResponse_Completion{Completion: &pb.CompletionInfo{FinishReason: "tool_calls"}}},
	}

	resp, err := UniversalToAnthropic("r", responses)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 期望 3 个 block：text("Hello world") + tool_use(f) + text("done")
	if len(resp.Content) != 3 {
		t.Fatalf("Expected 3 blocks (text+tool_use+text), got %d: %+v", len(resp.Content), resp.Content)
	}
	if resp.Content[0].Text != "Hello world" {
		t.Errorf("block0 text=%q, want 'Hello world'", resp.Content[0].Text)
	}
	if resp.Content[1].Type != "tool_use" || resp.Content[1].Name != "f" {
		t.Errorf("block1=%+v, want tool_use 'f'", resp.Content[1])
	}
	if resp.Content[2].Text != "done" {
		t.Errorf("block2 text=%q, want 'done'", resp.Content[2].Text)
	}
}

func TestUniversalToOpenAI(t *testing.T) {
	responses := []*pb.UniversalResponse{
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Chunk{
				Chunk: &pb.ContentChunk{Text: "Hello"},
			},
		},
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Completion{
				Completion: &pb.CompletionInfo{
					FinishReason: "stop",
					InputTokens:  10,
					OutputTokens: 20,
				},
			},
		},
	}

	openaiResp, err := UniversalToOpenAI("req_123", "gpt-4", responses)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证消息
	if len(openaiResp.Choices) != 1 {
		t.Errorf("Expected 1 choice")
	}

	if openaiResp.Choices[0].Message.Content != "Hello" {
		t.Errorf("Content not correct")
	}

	// 验证 token 统计
	if openaiResp.Usage.PromptTokens != 10 {
		t.Errorf("PromptTokens not correct")
	}
}

func TestResponsesToUniversalFunctionCallOutput(t *testing.T) {
	req := &protocol.ResponseRequest{
		Model: "gpt-4.1",
		Input: []byte(`[{"type":"function_call_output","call_id":"call_1","output":"done"}]`),
	}
	univReq, err := ResponsesToUniversal(req)
	if err != nil {
		t.Fatalf("ResponsesToUniversal failed: %v", err)
	}
	if len(univReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(univReq.Messages))
	}
	got := univReq.Messages[0].GetContent()[0].GetToolResult()
	if got == nil || got.ToolCallId != "call_1" || got.Result != "done" {
		t.Fatalf("function_call_output should become tool_result, got %+v", univReq.Messages[0].GetContent())
	}
}

func TestResponsesToUniversalFunctionCall(t *testing.T) {
	req := &protocol.ResponseRequest{
		Model: "gpt-4.1",
		Input: []byte(`[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"city\":\"SF\"}"}]`),
	}
	univReq, err := ResponsesToUniversal(req)
	if err != nil {
		t.Fatalf("ResponsesToUniversal failed: %v", err)
	}
	if len(univReq.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(univReq.Messages))
	}
	got := univReq.Messages[0].GetContent()[0].GetToolCall()
	if got == nil || got.Id != "call_1" || got.Name != "lookup" || got.Arguments != "{\"city\":\"SF\"}" {
		t.Fatalf("function_call should become tool_call, got %+v", univReq.Messages[0].GetContent())
	}
}

func TestUniversalToResponsesIncludesFunctionCallOutputItem(t *testing.T) {
	responses := []*pb.UniversalResponse{
		{Response: &pb.UniversalResponse_ToolCall{ToolCall: &pb.ToolCall{Id: "call_1", Name: "lookup", Arguments: `{"city":"SF"}`}}},
		{Response: &pb.UniversalResponse_Completion{Completion: &pb.CompletionInfo{FinishReason: "tool_calls", InputTokens: 3, OutputTokens: 5}}},
	}
	resp, err := UniversalToResponses("resp_1", "gpt-4.1", responses)
	if err != nil {
		t.Fatalf("UniversalToResponses failed: %v", err)
	}
	if len(resp.Output) != 1 {
		t.Fatalf("expected 1 output item, got %d", len(resp.Output))
	}
	item := resp.Output[0]
	if item.Type != "function_call" || item.CallID != "call_1" || item.Name != "lookup" || item.Arguments != `{"city":"SF"}` {
		t.Fatalf("unexpected output item: %+v", item)
	}
}

func TestUniversalToGemini(t *testing.T) {
	responses := []*pb.UniversalResponse{
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Chunk{
				Chunk: &pb.ContentChunk{Text: "Hello"},
			},
		},
		{
			RequestId: "req_123",
			Response: &pb.UniversalResponse_Completion{
				Completion: &pb.CompletionInfo{
					FinishReason: "stop",
					InputTokens:  10,
					OutputTokens: 20,
				},
			},
		},
	}

	geminiResp, err := UniversalToGemini(responses)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// 验证候选
	if len(geminiResp.Candidates) != 1 {
		t.Errorf("Expected 1 candidate")
	}

	// 验证内容
	if len(geminiResp.Candidates[0].Content.Parts) != 1 {
		t.Errorf("Expected 1 part")
	}

	// 验证 token 统计
	if geminiResp.UsageMetadata.PromptTokenCount != 10 {
		t.Errorf("PromptTokenCount not correct")
	}
}
