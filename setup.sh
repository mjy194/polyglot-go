#!/bin/bash

# Polyglot 快速启动脚本

set -e

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$BASE_DIR"

echo "🚀 Polyglot Quick Start"
echo ""

# ============================================================================
# 1. 生成 gRPC 代码
# ============================================================================
echo "📝 Step 1: Generating gRPC code..."
cd src/srv/proto

if ! command -v protoc &> /dev/null; then
    echo "❌ protoc not found. Please install Protocol Buffers compiler:"
    echo "   macOS: brew install protobuf"
    echo "   Linux: apt-get install protobuf-compiler"
    exit 1
fi

echo "   Installing Go plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo "   Generating code..."
./generate.sh

cd "$BASE_DIR"
echo "   ✅ gRPC code generated"
echo ""

# ============================================================================
# 2. 初始化后端
# ============================================================================
echo "📦 Step 2: Initializing backend..."
cd src/srv

if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Please install Go 1.21+"
    exit 1
fi

echo "   Downloading dependencies..."
go mod tidy

echo "   ✅ Backend initialized"
echo ""

cd "$BASE_DIR"

# ============================================================================
# 3. 初始化前端
# ============================================================================
echo "📦 Step 3: Initializing frontend..."
cd src/web

if ! command -v npm &> /dev/null; then
    echo "❌ npm not found. Please install Node.js"
    exit 1
fi

if [ ! -d "node_modules" ]; then
    echo "   Installing dependencies..."
    npm install
else
    echo "   Dependencies already installed"
fi

echo "   ✅ Frontend initialized"
echo ""

cd "$BASE_DIR"

# ============================================================================
# 4. 检查端口占用
# ============================================================================
echo "🔍 Step 4: Checking ports..."

check_port() {
    port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1 ; then
        echo "   ⚠️  Port $port is in use"
        return 1
    else
        echo "   ✅ Port $port is available"
        return 0
    fi
}

check_port 3000  # Backend
check_port 3001  # Frontend

echo ""

# ============================================================================
# 5. 启动选项
# ============================================================================
echo "✅ Setup complete!"
echo ""
echo "📋 Next steps:"
echo ""
echo "Option A: Start both services (recommended)"
echo "   ./dev.sh"
echo ""
echo "Option B: Start separately"
echo "   Terminal 1: cd src/srv && go run cmd/polyglot/main.go"
echo "   Terminal 2: cd src/web && npm run dev"
echo ""
echo "🌐 URLs:"
echo "   Backend:  http://localhost:3000"
echo "   Frontend: http://localhost:3001/ui/"
echo ""
