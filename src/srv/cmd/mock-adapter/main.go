package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	pb "polyglot/proto/adapter"

	"google.golang.org/grpc"
)

// MockAdapter 实现 AdapterService 接口
type MockAdapter struct {
	pb.UnimplementedAdapterServiceServer
	name    string
	version string
}

// NewMockAdapter 创建 Mock Adapter
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		name:    "mock-adapter",
		version: "1.0.0",
	}
}

// GetMetadata 返回 Adapter 元数据
func (a *MockAdapter) GetMetadata(ctx context.Context, req *pb.GetMetadataRequest) (*pb.AdapterMetadata, error) {
	log.Println("📊 GetMetadata called")

	return &pb.AdapterMetadata{
		Name:    a.name,
		Version: a.version,
		SupportedModels: []string{
			"claude-opus-4-8",
			"claude-sonnet-4-6",
			"gpt-4",
			"gemini-pro",
		},
		Capabilities: &pb.AdapterCapabilities{
			Streaming:     true,
			ToolUse:       true,
			Vision:        true,
			PromptCaching: false,
			MaxTokens:     100000,
			MaxConcurrent: 10,
		},
		NativeProtocols: []*pb.NativeProtocolSupport{
			{Protocol: "anthropic", Endpoints: []string{"messages"}, Streaming: true},
			{Protocol: "openai", Endpoints: []string{"chat_completions"}, Streaming: true},
			{Protocol: "responses", Endpoints: []string{"responses"}, Streaming: true},
			{Protocol: "gemini", Endpoints: []string{"generate_content"}, Streaming: true},
		},
		Metadata: map[string]string{
			"backend": "mock",
			"author":  "polyglot",
		},
	}, nil
}

// HealthCheck 健康检查
func (a *MockAdapter) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Println("❤️  HealthCheck called")

	return &pb.HealthCheckResponse{
		Status:         pb.HealthCheckResponse_HEALTHY,
		Message:        "Mock adapter is healthy",
		UptimeSeconds:  3600,
		ActiveRequests: 0,
		ErrorRate:      0,
		Metrics: map[string]string{
			"requests_total": "100",
			"latency_avg":    "50ms",
		},
	}, nil
}

// ProcessRequest 处理请求（流式）
func (a *MockAdapter) ProcessRequest(req *pb.UniversalRequest, stream pb.AdapterService_ProcessRequestServer) error {
	log.Printf("🔄 ProcessRequest called: request_id=%s, model=%s\n", req.RequestId, req.Model)

	// 提取用户消息
	var userMessage string
	if len(req.Messages) > 0 {
		for _, msg := range req.Messages {
			if msg.Role == pb.Message_USER {
				for _, part := range msg.Content {
					if textPart := part.GetText(); textPart != nil {
						userMessage = textPart.Text
						break
					}
				}
			}
		}
	}

	// 检查是否有工具
	hasTool := len(req.Tools) > 0

	// 模拟处理延迟
	time.Sleep(100 * time.Millisecond)

	if hasTool {
		// 返回工具调用
		log.Printf("🛠️  Returning tool call for: %s\n", userMessage)

		// 工具调用
		if err := stream.Send(&pb.UniversalResponse{
			RequestId: req.RequestId,
			Response: &pb.UniversalResponse_ToolCall{
				ToolCall: &pb.ToolCall{
					Id:        "call_mock_123",
					Name:      req.Tools[0].Name,
					Arguments: `{"location": "San Francisco"}`,
					Index:     0,
				},
			},
		}); err != nil {
			return err
		}

		time.Sleep(50 * time.Millisecond)
	} else {
		// 返回流式文本响应
		log.Printf("💬 Streaming text response for: %s\n", userMessage)

		// 分块发送响应
		chunks := []string{
			"Hello! ",
			"This is ",
			"a mock ",
			"response ",
			"from ",
			"the adapter. ",
			fmt.Sprintf("You said: \"%s\"", userMessage),
		}

		for i, chunk := range chunks {
			if err := stream.Send(&pb.UniversalResponse{
				RequestId: req.RequestId,
				Response: &pb.UniversalResponse_Chunk{
					Chunk: &pb.ContentChunk{
						Text:    chunk,
						Index:   int32(i),
						IsFinal: i == len(chunks)-1,
					},
				},
			}); err != nil {
				return err
			}

			// 模拟流式延迟
			time.Sleep(50 * time.Millisecond)
		}
	}

	// 发送完成信息
	if err := stream.Send(&pb.UniversalResponse{
		RequestId: req.RequestId,
		Response: &pb.UniversalResponse_Completion{
			Completion: &pb.CompletionInfo{
				FinishReason:        "stop",
				InputTokens:         20,
				OutputTokens:        50,
				DurationMs:          300,
				CacheCreationTokens: 0,
				CacheReadTokens:     0,
			},
		},
	}); err != nil {
		return err
	}

	log.Printf("✅ Request completed: %s\n", req.RequestId)
	return nil
}

func (a *MockAdapter) ProcessNative(req *pb.NativeRequest, stream pb.AdapterService_ProcessNativeServer) error {
	log.Printf("🔄 ProcessNative called: request_id=%s, protocol=%s, endpoint=%s\n", req.RequestId, req.Protocol, req.Endpoint)

	contentType := "application/json"
	body := []byte(fmt.Sprintf(`{"native":true,"protocol":%q,"endpoint":%q}`, req.Protocol, req.Endpoint))
	if req.Stream {
		contentType = "text/event-stream"
		body = []byte(fmt.Sprintf("data: {\"native\":true,\"protocol\":%q,\"endpoint\":%q}\n\n", req.Protocol, req.Endpoint))
	}

	return stream.Send(&pb.NativeResponse{
		RequestId:  req.RequestId,
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": contentType},
		Body:       body,
		EndStream:  true,
	})
}

// CancelRequest 取消请求
func (a *MockAdapter) CancelRequest(ctx context.Context, req *pb.CancelRequestRequest) (*pb.CancelRequestResponse, error) {
	log.Printf("❌ CancelRequest called: %s\n", req.RequestId)

	return &pb.CancelRequestResponse{
		Cancelled: true,
		Message:   "Request cancelled (mock)",
	}, nil
}

func main() {
	// 监听端口
	port := 50051
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// 创建 gRPC 服务器
	grpcServer := grpc.NewServer()

	// 注册服务
	adapter := NewMockAdapter()
	pb.RegisterAdapterServiceServer(grpcServer, adapter)

	log.Printf("🚀 Mock Adapter started on port %d\n", port)
	log.Println("📡 Waiting for connections...")

	// 启动服务
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
