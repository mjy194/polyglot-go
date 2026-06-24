# Polyglot 快速参考

## 🚀 快速开始

```bash
cd /home/javion/work/tools/polyglot/new/polyglot

# 首次运行
./setup.sh      # 初始化（生成代码 + 安装依赖）
./dev.sh        # 启动开发服务器

# 访问
# http://localhost:3001/ui/  - 前端界面
# http://localhost:3000/api/ - 后端 API
```

## 📋 常用命令

### 开发服务器

```bash
./dev.sh              # 启动前后端
./dev.sh stop         # 停止服务
./dev.sh restart      # 重启服务
./dev.sh status       # 查看状态
./dev.sh logs         # 查看日志
```

### 开发工作流

```bash
# 后端开发
cd src/srv
go run cmd/polyglot/main.go      # 运行
go test ./...                     # 测试
go fmt ./...                      # 格式化

# 前端开发
cd src/web
npm run dev                       # 开发服务器（热重载）
npm run build                     # 构建生产版本
npm run preview                   # 预览构建

# 生成 gRPC 代码
cd src/srv/proto
./generate.sh
```

### 日志查看

```bash
tail -f logs/backend.log          # 后端日志
tail -f logs/frontend.log         # 前端日志
./dev.sh logs                     # 同时查看两个
```

## 📁 项目结构

```
new/polyglot/
├── src/
│   ├── srv/              # Go 后端
│   │   ├── cmd/          # 入口
│   │   ├── internal/     # 业务代码
│   │   ├── pkg/          # 公共库
│   │   ├── proto/        # gRPC 定义
│   │   └── configs/      # 配置
│   └── web/              # React 前端
│       ├── src/
│       │   ├── pages/    # 页面
│       │   ├── components/
│       │   ├── api/
│       │   └── styles/
│       └── public/
├── setup.sh              # 初始化
└── dev.sh                # 开发服务器
```

## 🔧 配置文件

| 文件 | 说明 |
|------|------|
| `src/srv/configs/config.yaml` | 后端配置 |
| `src/srv/go.mod` | Go 依赖 |
| `src/web/package.json` | 前端依赖 |
| `src/web/vite.config.ts` | Vite 配置 |

## 📝 核心文件

| 文件 | 说明 |
|------|------|
| `src/srv/proto/adapter/adapter.proto` | gRPC 协议 |
| `src/srv/internal/adapter/adapter.go` | Adapter 接口 |
| `src/srv/internal/server/server.go` | HTTP 服务器 |
| `src/web/src/App.tsx` | React 主应用 |
| `src/web/src/api/client.ts` | API 客户端 |

## 🌐 端口

| 服务 | 端口 | URL |
|------|------|-----|
| 后端 | 3000 | http://localhost:3000 |
| 前端 | 3001 | http://localhost:3001/ui/ |
| Adapter | 50051 | localhost:50051 (gRPC) |

## 📚 文档

| 文档 | 说明 |
|------|------|
| `GETTING_STARTED.md` | 快速开始 |
| `FINAL_SUMMARY.md` | 完整总结 |
| `PROJECT_STRUCTURE.md` | 项目结构 |

## 🐛 故障排查

### 后端启动失败

```bash
# 查看日志
cat logs/backend.log

# 检查端口占用
lsof -i :3000

# 重新生成 gRPC 代码
cd src/srv/proto && ./generate.sh
```

### 前端启动失败

```bash
# 查看日志
cat logs/frontend.log

# 重新安装依赖
cd src/web
rm -rf node_modules package-lock.json
npm install
```

### 端口被占用

```bash
# 查找占用进程
lsof -i :3000
lsof -i :3001

# 停止 Polyglot 服务
./dev.sh stop

# 或强制杀死进程
kill -9 $(lsof -t -i:3000)
kill -9 $(lsof -t -i:3001)
```

## ⚡ 快捷技巧

```bash
# 一键重启
./dev.sh restart

# 查看运行状态
./dev.sh status

# 实时查看日志
./dev.sh logs

# 只运行后端
cd src/srv && go run cmd/polyglot/main.go

# 只运行前端
cd src/web && npm run dev
```

## 🎯 下一步

1. 实现 gRPC 客户端 (`internal/adapter/grpc_adapter.go`)
2. 实现路由器 (`internal/server/router/`)
3. 创建 Adapter 独立仓库
4. 完善前端数据展示

---

**需要帮助？** 查看完整文档：
- `GETTING_STARTED.md`
- `../migration/GRPC_CONFIG_ROUTING.md`
