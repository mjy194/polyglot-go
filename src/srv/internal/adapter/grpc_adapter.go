package adapter

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "polyglot/proto/adapter"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCAdapter gRPC 客户端适配器
type GRPCAdapter struct {
	name     string
	address  string
	conn     *grpc.ClientConn
	client   pb.AdapterServiceClient
	metadata *pb.AdapterMetadata
}

// NewGRPCAdapter 创建 gRPC 适配器
func NewGRPCAdapter(name, address string) (*GRPCAdapter, error) {
	log.Printf("🔌 Connecting to adapter: %s at %s\n", name, address)

	// 连接到 gRPC 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	client := pb.NewAdapterServiceClient(conn)

	adapter := &GRPCAdapter{
		name:    name,
		address: address,
		conn:    conn,
		client:  client,
	}

	// 获取 Metadata
	if err := adapter.fetchMetadata(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	log.Printf("✅ Connected to adapter: %s (version %s)\n", adapter.metadata.Name, adapter.metadata.Version)
	log.Printf("   Supported models: %v\n", adapter.metadata.SupportedModels)

	return adapter, nil
}

// fetchMetadata 获取 Adapter 元数据
func (a *GRPCAdapter) fetchMetadata() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	metadata, err := a.client.GetMetadata(ctx, &pb.GetMetadataRequest{})
	if err != nil {
		return err
	}

	a.metadata = metadata
	return nil
}

// Name 返回适配器名称
func (a *GRPCAdapter) Name() string {
	if a.metadata != nil {
		return a.metadata.Name
	}
	return a.name
}

// GetMetadata 获取元数据
func (a *GRPCAdapter) GetMetadata() *pb.AdapterMetadata {
	return a.metadata
}

// HealthCheck 健康检查
func (a *GRPCAdapter) HealthCheck(ctx context.Context) (*pb.HealthCheckResponse, error) {
	return a.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
}

// ProcessRequest 处理请求
func (a *GRPCAdapter) ProcessRequest(ctx context.Context, req *pb.UniversalRequest) ([]*pb.UniversalResponse, error) {
	log.Printf("📤 Sending request to adapter: %s (model=%s)\n", a.Name(), req.Model)

	// 调用 gRPC
	stream, err := a.client.ProcessRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ProcessRequest: %w", err)
	}

	// 接收流式响应
	var responses []*pb.UniversalResponse
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("stream error: %w", err)
		}

		responses = append(responses, resp)

		// 日志
		switch r := resp.Response.(type) {
		case *pb.UniversalResponse_Chunk:
			log.Printf("📥 Chunk: %q (index=%d, final=%v)\n", r.Chunk.Text, r.Chunk.Index, r.Chunk.IsFinal)
		case *pb.UniversalResponse_ToolCall:
			log.Printf("🛠️  Tool call: %s (id=%s)\n", r.ToolCall.Name, r.ToolCall.Id)
		case *pb.UniversalResponse_Error:
			log.Printf("❌ Error: %s (code=%d)\n", r.Error.Message, r.Error.Code)
		case *pb.UniversalResponse_Completion:
			log.Printf("✅ Completion: %s (input=%d, output=%d)\n",
				r.Completion.FinishReason, r.Completion.InputTokens, r.Completion.OutputTokens)
		}
	}

	log.Printf("✅ Received %d responses from adapter\n", len(responses))
	return responses, nil
}

// CancelRequest 取消请求
func (a *GRPCAdapter) CancelRequest(ctx context.Context, requestID string) error {
	resp, err := a.client.CancelRequest(ctx, &pb.CancelRequestRequest{
		RequestId: requestID,
	})
	if err != nil {
		return err
	}

	if !resp.Cancelled {
		return fmt.Errorf("cancel failed: %s", resp.Message)
	}

	return nil
}

// Close 关闭连接
func (a *GRPCAdapter) Close() error {
	if a.conn != nil {
		log.Printf("🔌 Closing connection to adapter: %s\n", a.Name())
		return a.conn.Close()
	}
	return nil
}

// SupportsModel 检查是否支持模型
func (a *GRPCAdapter) SupportsModel(model string) bool {
	if a.metadata == nil {
		return false
	}

	for _, m := range a.metadata.SupportedModels {
		if m == model {
			return true
		}
	}

	return false
}

// SupportsCapability 检查是否支持能力
func (a *GRPCAdapter) SupportsCapability(capability string) bool {
	if a.metadata == nil || a.metadata.Capabilities == nil {
		return false
	}

	caps := a.metadata.Capabilities
	switch capability {
	case "streaming":
		return caps.Streaming
	case "tool_use":
		return caps.ToolUse
	case "vision":
		return caps.Vision
	case "prompt_caching":
		return caps.PromptCaching
	default:
		return false
	}
}
