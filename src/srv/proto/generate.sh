#!/bin/bash

# Proto 代码生成脚本

set -e

cd "$(dirname "$0")"

echo "🔧 Generating protobuf code..."

# 检查 protoc 是否安装
if ! command -v protoc &> /dev/null; then
    echo "❌ protoc not found. Please install:"
    echo "   brew install protobuf (macOS)"
    echo "   apt-get install protobuf-compiler (Linux)"
    exit 1
fi

# 安装 Go protobuf 插件（如果未安装）
echo "📦 Installing Go protobuf plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成 Go 代码
echo "🔨 Generating Go code from adapter.proto..."
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    adapter/adapter.proto

echo ""
echo "✅ Code generation complete!"
echo "📄 Generated files:"
echo "   - adapter/adapter.pb.go"
echo "   - adapter/adapter_grpc.pb.go"
