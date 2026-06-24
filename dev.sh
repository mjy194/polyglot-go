#!/bin/bash

# Polyglot 开发脚本
# 用法：
#   ./dev.sh          - 启动开发服务器
#   ./dev.sh stop     - 停止服务器

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$BASE_DIR/.dev.pids"
ADAPTER_DIR="$BASE_DIR/../uipath_adapter"
ENV_FILE="$BASE_DIR/.env"

# ============================================================================
# 停止服务
# ============================================================================
stop_services() {
    echo "🛑 Stopping Polyglot development servers..."
    echo ""

    if [ ! -f "$PID_FILE" ]; then
        echo "No running services found."
        return
    fi

    # 读取 PIDs
    while IFS='=' read -r name pid; do
        if [ -n "$pid" ] && ps -p $pid > /dev/null 2>&1; then
            echo "Stopping $name (PID: $pid)..."
            kill $pid 2>/dev/null
            sleep 1

            # 强制杀死
            if ps -p $pid > /dev/null 2>&1; then
                kill -9 $pid 2>/dev/null
            fi
            echo "   ✅ $name stopped"
        fi
    done < "$PID_FILE"

    rm -f "$PID_FILE"
    echo ""
    echo "✅ All services stopped"
}

# ============================================================================
# 启动服务
# ============================================================================
start_services() {
    echo "🚀 Starting Polyglot development servers..."
    echo ""

    # 检查是否已运行
    if [ -f "$PID_FILE" ]; then
        echo "⚠️  Services may already be running."
        echo "   Run './dev.sh stop' first, or check .dev.pids"
        echo ""
        read -p "Stop existing services and restart? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            stop_services
            echo ""
        else
            exit 0
        fi
    fi

    # 创建 logs 目录
    mkdir -p logs

    # 启动后端
    echo "📡 Starting backend server..."
    cd "$BASE_DIR/src/srv"
    go run cmd/polyglot/main.go > "$BASE_DIR/logs/backend.log" 2>&1 &
    BACKEND_PID=$!
    echo "BACKEND=$BACKEND_PID" > "$PID_FILE"
    echo "   Backend PID: $BACKEND_PID"
    echo "   Logs: logs/backend.log"

    # 等待后端启动
    echo "   Waiting for backend to start..."
    sleep 2

    # 检查后端是否启动成功
    if ! ps -p $BACKEND_PID > /dev/null; then
        echo "   ❌ Backend failed to start. Check logs/backend.log"
        cat "$BASE_DIR/logs/backend.log" | tail -20
        rm -f "$PID_FILE"
        exit 1
    fi
    echo "   ✅ Backend started"

    # 启动 UiPath adapter
    echo ""
    echo "🔌 Starting uipath_adapter..."
    if [ ! -d "$ADAPTER_DIR" ]; then
        echo "   ⚠️  Adapter dir not found: $ADAPTER_DIR — skipping"
    elif [ ! -f "$ENV_FILE" ]; then
        echo "   ⚠️  $ENV_FILE not found — adapter will run in mock mode"
        echo "   Create .env with UIPATH_EMAIL/PASSWORD/ORG_NAME/TENANT_NAME for real backend"
    else
        # 加载 env 并启动 adapter（仅本子 shell 见到这些 env，不污染当前 shell）
        (
            set -a
            . "$ENV_FILE"
            set +a
            cd "$ADAPTER_DIR"
            exec go run ./cmd/adapter
        ) > "$BASE_DIR/logs/adapter.log" 2>&1 &
        ADAPTER_PID=$!
        echo "ADAPTER=$ADAPTER_PID" >> "$PID_FILE"
        echo "   Adapter PID: $ADAPTER_PID"
        echo "   Logs: logs/adapter.log"

        echo "   Waiting for adapter to register..."
        sleep 3

        if ! ps -p $ADAPTER_PID > /dev/null; then
            echo "   ❌ Adapter failed to start. Last 20 lines of logs/adapter.log:"
            tail -20 "$BASE_DIR/logs/adapter.log"
            kill $BACKEND_PID 2>/dev/null
            rm -f "$PID_FILE"
            exit 1
        fi
        echo "   ✅ Adapter started"
    fi

    # 启动前端
    echo ""
    echo "🌐 Starting frontend server..."
    cd "$BASE_DIR/src/web"
    npm run dev < /dev/null > "$BASE_DIR/logs/frontend.log" 2>&1 &
    FRONTEND_PID=$!
    echo "FRONTEND=$FRONTEND_PID" >> "$PID_FILE"
    echo "   Frontend PID: $FRONTEND_PID"
    echo "   Logs: logs/frontend.log"

    # 等待前端启动
    echo "   Waiting for frontend to start..."
    sleep 3

    # 检查前端是否启动成功
    if ! ps -p $FRONTEND_PID > /dev/null; then
        echo "   ❌ Frontend failed to start. Check logs/frontend.log"
        cat "$BASE_DIR/logs/frontend.log" | tail -20
        kill $BACKEND_PID 2>/dev/null
        rm -f "$PID_FILE"
        exit 1
    fi
    echo "   ✅ Frontend started"

    echo ""
    echo "✅ Polyglot development servers started!"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🌐 URLs:"
    echo "   Frontend:  http://localhost:3001/"
    echo "   Backend:   http://localhost:3100  (gRPC :50052)"
    echo "   API:       http://localhost:3100/api/"
    echo "   Adapter:   gRPC :50051 (uipath_adapter)"
    echo ""
    echo "📝 Logs:"
    echo "   Backend:   tail -f logs/backend.log"
    echo "   Frontend:  tail -f logs/frontend.log"
    echo ""
    echo "🛑 Stop:"
    echo "   ./dev.sh stop"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Press Ctrl+C to stop (will keep services running in background)"
    echo "Or run './dev.sh stop' to stop services"
    echo ""

    # 可选：保持脚本运行并监控
    # trap "echo ''; echo 'Use ./dev.sh stop to stop services'; exit" INT TERM
    # tail -f logs/backend.log logs/frontend.log
}

# ============================================================================
# 主逻辑
# ============================================================================
case "${1:-start}" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        echo ""
        start_services
        ;;
    status)
        if [ -f "$PID_FILE" ]; then
            echo "📊 Service Status:"
            echo ""
            while IFS='=' read -r name pid; do
                if [ -n "$pid" ] && ps -p $pid > /dev/null 2>&1; then
                    echo "✅ $name (PID: $pid) - Running"
                else
                    echo "❌ $name (PID: $pid) - Not running"
                fi
            done < "$PID_FILE"
        else
            echo "No services running"
        fi
        ;;
    logs)
        if [ -f "$PID_FILE" ]; then
            tail -f logs/backend.log logs/frontend.log
        else
            echo "No services running"
        fi
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        echo ""
        echo "Commands:"
        echo "  start    - Start development servers (default)"
        echo "  stop     - Stop development servers"
        echo "  restart  - Restart development servers"
        echo "  status   - Show service status"
        echo "  logs     - Tail log files"
        exit 1
        ;;
esac
