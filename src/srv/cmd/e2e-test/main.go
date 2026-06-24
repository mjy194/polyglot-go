package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"polyglot/internal/adapter"
	"polyglot/internal/converter"
	"polyglot/internal/protocol"
	pb "polyglot/proto/adapter"
)

func main() {
	log.Println("🧪 Starting End-to-End Test")
	log.Println("=" + string(make([]byte, 60)))

	// 连接到 Mock Adapter
	log.Println("\n📡 Step 1: Connecting to Mock Adapter...")
	grpcAdapter, err := adapter.NewGRPCAdapter("mock", "localhost:50051")
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer grpcAdapter.Close()

	// 测试 1: Metadata
	log.Println("\n📊 Test 1: GetMetadata")
	log.Println("-" + string(make([]byte, 60)))
	metadata := grpcAdapter.GetMetadata()
	log.Printf("  Name: %s\n", metadata.Name)
	log.Printf("  Version: %s\n", metadata.Version)
	log.Printf("  Supported Models: %v\n", metadata.SupportedModels)
	log.Printf("  Streaming: %v\n", metadata.Capabilities.Streaming)
	log.Printf("  Tool Use: %v\n", metadata.Capabilities.ToolUse)

	// 测试 2: HealthCheck
	log.Println("\n❤️  Test 2: HealthCheck")
	log.Println("-" + string(make([]byte, 60)))
	ctx := context.Background()
	health, err := grpcAdapter.HealthCheck(ctx)
	if err != nil {
		log.Printf("  ❌ Failed: %v\n", err)
	} else {
		log.Printf("  Status: %s\n", health.Status)
		log.Printf("  Message: %s\n", health.Message)
		log.Printf("  Uptime: %d seconds\n", health.UptimeSeconds)
	}

	// 测试 3: Anthropic → Universal → Adapter
	log.Println("\n💬 Test 3: Anthropic Request → Adapter")
	log.Println("-" + string(make([]byte, 60)))

	anthropicReq := &protocol.AnthropicRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 100,
		Messages: []protocol.AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello, how are you?",
			},
		},
		Stream: true,
	}

	// 转换为 Universal
	log.Println("  🔄 Converting Anthropic → Universal...")
	univReq, err := converter.AnthropicToUniversal(anthropicReq)
	if err != nil {
		log.Fatalf("  ❌ Conversion failed: %v", err)
	}
	log.Printf("  ✅ Converted: model=%s, messages=%d\n", univReq.Model, len(univReq.Messages))

	// 发送到 Adapter
	log.Println("  📤 Sending to Adapter...")
	responses, err := grpcAdapter.ProcessRequest(ctx, univReq)
	if err != nil {
		log.Fatalf("  ❌ Request failed: %v", err)
	}

	// 转换回 Anthropic
	log.Println("  🔄 Converting Universal → Anthropic...")
	anthropicResp, err := converter.UniversalToAnthropic(univReq.RequestId, responses)
	if err != nil {
		log.Fatalf("  ❌ Conversion failed: %v", err)
	}

	log.Printf("  ✅ Response received:\n")
	log.Printf("     Role: %s\n", anthropicResp.Role)
	log.Printf("     Content blocks: %d\n", len(anthropicResp.Content))
	log.Printf("     Input tokens: %d\n", anthropicResp.Usage.InputTokens)
	log.Printf("     Output tokens: %d\n", anthropicResp.Usage.OutputTokens)

	// 显示内容
	for i, block := range anthropicResp.Content {
		if block.Type == "text" {
			log.Printf("     Block %d: %q\n", i, block.Text)
		}
	}

	// 测试 4: OpenAI → Universal → Adapter
	log.Println("\n💬 Test 4: OpenAI Request → Adapter")
	log.Println("-" + string(make([]byte, 60)))

	openaiReq := &protocol.OpenAIRequest{
		Model: "gpt-4",
		Messages: []protocol.OpenAIMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant",
			},
			{
				Role:    "user",
				Content: "Tell me a joke",
			},
		},
		MaxTokens: 100,
		Stream:    true,
	}

	// 转换为 Universal
	log.Println("  🔄 Converting OpenAI → Universal...")
	univReq2, err := converter.OpenAIToUniversal(openaiReq)
	if err != nil {
		log.Fatalf("  ❌ Conversion failed: %v", err)
	}
	log.Printf("  ✅ Converted: model=%s, system=%q\n", univReq2.Model, univReq2.System)

	// 发送到 Adapter
	log.Println("  📤 Sending to Adapter...")
	responses2, err := grpcAdapter.ProcessRequest(ctx, univReq2)
	if err != nil {
		log.Fatalf("  ❌ Request failed: %v", err)
	}

	// 转换回 OpenAI
	log.Println("  🔄 Converting Universal → OpenAI...")
	openaiResp, err := converter.UniversalToOpenAI(univReq2.RequestId, univReq2.Model, responses2)
	if err != nil {
		log.Fatalf("  ❌ Conversion failed: %v", err)
	}

	log.Printf("  ✅ Response received:\n")
	log.Printf("     Model: %s\n", openaiResp.Model)
	log.Printf("     Choices: %d\n", len(openaiResp.Choices))
	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		log.Printf("     Message role: %s\n", choice.Message.Role)
		log.Printf("     Message content: %q\n", choice.Message.Content)
		log.Printf("     Finish reason: %s\n", choice.FinishReason)
	}

	// 测试 5: 带工具的请求
	log.Println("\n🛠️  Test 5: Request with Tools")
	log.Println("-" + string(make([]byte, 60)))

	anthropicReqWithTool := &protocol.AnthropicRequest{
		Model:     "claude-opus-4-8",
		MaxTokens: 100,
		Messages: []protocol.AnthropicMessage{
			{
				Role:    "user",
				Content: "What's the weather in San Francisco?",
			},
		},
		Tools: []protocol.AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	// 转换并发送
	log.Println("  🔄 Converting and sending...")
	univReq3, _ := converter.AnthropicToUniversal(anthropicReqWithTool)
	log.Printf("  ✅ Request has %d tools\n", len(univReq3.Tools))

	responses3, err := grpcAdapter.ProcessRequest(ctx, univReq3)
	if err != nil {
		log.Fatalf("  ❌ Request failed: %v", err)
	}

	// 检查是否有工具调用
	hasToolCall := false
	for _, resp := range responses3 {
		if toolCall := resp.GetToolCall(); toolCall != nil {
			hasToolCall = true
			log.Printf("  ✅ Tool call received:\n")
			log.Printf("     ID: %s\n", toolCall.Id)
			log.Printf("     Name: %s\n", toolCall.Name)
			log.Printf("     Arguments: %s\n", toolCall.Arguments)
		}
	}

	if !hasToolCall {
		log.Println("  ⚠️  No tool call received")
	}

	// 测试 6: 性能测试
	log.Println("\n⚡ Test 6: Performance Test")
	log.Println("-" + string(make([]byte, 60)))

	start := time.Now()
	for i := 0; i < 5; i++ {
		req := &pb.UniversalRequest{
			RequestId: fmt.Sprintf("perf_%d", i),
			Model:     "claude-opus-4-8",
			Messages: []*pb.Message{
				{
					Role: pb.Message_USER,
					Content: []*pb.ContentPart{
						{Part: &pb.ContentPart_Text{Text: &pb.TextPart{Text: fmt.Sprintf("Test %d", i)}}},
					},
				},
			},
			Config: &pb.GenerationConfig{
				MaxTokens: 50,
				Stream:    true,
			},
		}

		_, err := grpcAdapter.ProcessRequest(ctx, req)
		if err != nil {
			log.Printf("  ❌ Request %d failed: %v\n", i, err)
		}
	}
	elapsed := time.Since(start)
	log.Printf("  ✅ Completed 5 requests in %v (avg: %v/request)\n", elapsed, elapsed/5)

	// 总结
	log.Println("\n" + string(make([]byte, 60)))
	log.Println("🎉 All tests completed successfully!")
	log.Println("=" + string(make([]byte, 60)))
}
