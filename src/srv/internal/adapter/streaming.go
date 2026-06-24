package adapter

import (
	"context"
	"fmt"
	"io"

	pb "polyglot/proto/adapter"
)

// StreamProcessor 把一条 universal 请求流式地发给某个 adapter，
// 并把收到的每条 UniversalResponse 经 onResp 回调交还调用方，直到流结束。
//
// 抽象成接口是为了让 HTTP handler 能在不依赖真实 gRPC 连接的前提下做单测
// （注入一个假实现即可）。
type StreamProcessor interface {
	ProcessStream(ctx context.Context, req *pb.UniversalRequest, onResp func(*pb.UniversalResponse) error) error
}

// NativeProcessor 把一条原生协议请求流式地发给 adapter，保持 HTTP/SSE wire 格式。
type NativeProcessor interface {
	ProcessNative(ctx context.Context, req *pb.NativeRequest, onResp func(*pb.NativeResponse) error) error
}

// adapterClientStream 基于 AdapterServiceClient 实现 StreamProcessor。
type adapterClientStream struct {
	client pb.AdapterServiceClient
}

// NewStreamProcessor 用已有的 AdapterServiceClient 构造 StreamProcessor。
// 不发起新的连接——client 通常复用 adapter 注册时建立的那条 gRPC 连接。
func NewStreamProcessor(client pb.AdapterServiceClient) StreamProcessor {
	return adapterClientStream{client: client}
}

// NewNativeProcessor 用已有的 AdapterServiceClient 构造 NativeProcessor。
func NewNativeProcessor(client pb.AdapterServiceClient) NativeProcessor {
	return adapterClientStream{client: client}
}

func (a adapterClientStream) ProcessStream(ctx context.Context, req *pb.UniversalRequest, onResp func(*pb.UniversalResponse) error) error {
	stream, err := a.client.ProcessRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("open adapter stream: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("adapter stream recv: %w", err)
		}
		if err := onResp(resp); err != nil {
			return err
		}
	}
}

func (a adapterClientStream) ProcessNative(ctx context.Context, req *pb.NativeRequest, onResp func(*pb.NativeResponse) error) error {
	stream, err := a.client.ProcessNative(ctx, req)
	if err != nil {
		return fmt.Errorf("open native adapter stream: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("native adapter stream recv: %w", err)
		}
		if err := onResp(resp); err != nil {
			return err
		}
	}
}

// SupportsNative reports whether metadata declares native support for a protocol endpoint.
func SupportsNative(metadata *pb.AdapterMetadata, protocol, endpoint string) bool {
	if metadata == nil {
		return false
	}
	for _, support := range metadata.GetNativeProtocols() {
		if support.GetProtocol() != protocol {
			continue
		}
		for _, candidate := range support.GetEndpoints() {
			if candidate == "*" || candidate == endpoint {
				return true
			}
		}
	}
	return false
}
