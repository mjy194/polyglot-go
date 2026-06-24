# Polyglot

**Universal AI API Gateway**

一个统一的 AI API 网关，支持多种协议和后端适配器。

---

## ✨ 特性

- 🔌 **多协议支持** - Anthropic、OpenAI、Gemini
- 🎯 **智能路由** - 按协议、模型、优先级路由
- 🔄 **流式响应** - 原生支持 SSE 流式输出
- 🏗️ **插件架构** - gRPC 适配器系统
- 🎨 **管理界面** - React 前端管理后台
- ⚡ **高性能** - Go 后端 + gRPC 通信

---

## 🚀 快速开始

### 初始化

```bash
# 克隆或进入项目目录
cd /home/javion/work/tools/polyglot/new/polyglot

# 初始化（首次运行）
./setup.sh
```

### 启动

```bash
# 启动开发服务器
./dev.sh

# 访问
# 前端: http://localhost:3001/ui/
# 后端: http://localhost:3000/api/
```

### 停止

```bash
./dev.sh stop
```

---

## 📋 常用命令

```bash
./dev.sh              # 启动开发服务器
./dev.sh stop         # 停止服务
./dev.sh restart      # 重启服务
./dev.sh status       # 查看状态
./dev.sh logs         # 查看日志
```

---

## 🏗️ 架构

```
┌─────────────────────────┐
│   React Frontend        │
│   (TypeScript + Vite)   │
│   :3001/ui/             │
└───────────┬─────────────┘
            │ HTTP/REST
            ▼
┌─────────────────────────┐
│   Go Backend            │
│   (Gin + gRPC Client)   │
│   :3000                 │
└───────────┬─────────────┘
            │ gRPC
            ▼
┌─────────────────────────┐
│   Adapters              │
│   (独立进程)             │
│   :50051+               │
└─────────────────────────┘
```

---

## 📁 项目结构

```
new/polyglot/
├── src/
│   ├── srv/              # Go 后端
│   │   ├── cmd/          # 主程序
│   │   ├── internal/     # 业务代码
│   │   ├── pkg/          # 公共库
│   │   ├── proto/        # gRPC 定义
│   │   └── configs/      # 配置
│   └── web/              # React 前端
│       ├── src/          # 源代码
│       └── public/       # 静态资源
├── setup.sh              # 初始化脚本
└── dev.sh                # 开发脚本
```

---

## 🔧 配置

主配置文件：`src/srv/configs/config.yaml`

```yaml
server:
  port: 3000

adapters:
  - name: uipath
    connection:
      address: localhost:50051
    routing:
      protocols: [anthropic, openai]
      priority: 100
```

---

## 🎯 路由策略

### 按协议路由

```yaml
routing:
  protocols: [anthropic, openai, gemini]
```

### 按模型路由

```yaml
routing:
  models: [claude-opus-4-8, gpt-4]
```

### 优先级和负载均衡

```yaml
routing:
  priority: 100    # 数字越大优先级越高
  weight: 2        # 负载均衡权重
```

---

## 🛠️ 开发

### 后端开发

```bash
cd src/srv

# 运行
go run cmd/polyglot/main.go

# 测试
go test ./...

# 生成 gRPC 代码
cd proto && ./generate.sh
```

### 前端开发

```bash
cd src/web

# 开发服务器（热重载）
npm run dev

# 构建
npm run build

# 预览
npm run preview
```

---

## 📚 文档

- [快速参考](./docs/QUICK_REFERENCE.md) - 常用命令和技巧
- [入门指南](./docs/GETTING_STARTED.md) - 详细的开始教程
- [完整总结](./docs/FINAL_SUMMARY.md) - 项目全面总结
- [项目结构](./docs/PROJECT_STRUCTURE.md) - 目录结构说明
- [配置和路由](../migration/GRPC_CONFIG_ROUTING.md) - 配置管理详解
- [多仓库架构](../migration/MULTI_REPO_ARCHITECTURE.md) - 架构设计文档

---

## 🎨 技术栈

### 后端
- **语言**: Go 1.21+
- **框架**: Gin (HTTP), gRPC
- **配置**: Viper (YAML)
- **日志**: Zap

### 前端
- **语言**: TypeScript
- **框架**: React 18
- **构建**: Vite
- **路由**: React Router v6
- **HTTP**: Axios

### 协议
- **客户端 → Core**: HTTP/JSON (RESTful)
- **Core → Adapter**: gRPC (Protobuf)
- **流式**: SSE (Server-Sent Events)

---

## 🔌 Adapter 开发

创建独立的 Adapter 仓库实现 gRPC 服务：

```protobuf
service AdapterService {
  rpc GetMetadata() returns (AdapterMetadata);
  rpc HealthCheck() returns (HealthCheckResponse);
  rpc ProcessRequest() returns (stream ProcessRequestResponse);
  rpc CancelRequest() returns (CancelRequestResponse);
}
```

详见：`src/srv/proto/adapter/adapter.proto`

---

## 🐛 故障排查

### 端口被占用

```bash
# 查看占用
lsof -i :3000
lsof -i :3001

# 停止服务
./dev.sh stop
```

### 重新初始化

```bash
# 清理并重新安装
cd src/srv && go clean -modcache && go mod tidy
cd src/web && rm -rf node_modules && npm install

# 重新生成 gRPC 代码
cd src/srv/proto && ./generate.sh
```

---

## 📊 项目状态

- ✅ 项目结构
- ✅ gRPC 协议定义
- ✅ Go 后端框架
- ✅ React 前端框架
- 🔧 gRPC 客户端实现（进行中）
- 🔧 路由器实现（计划中）
- 🔧 Adapter 实现（计划中）

---

## 🤝 贡献

欢迎贡献！请查看 [CONTRIBUTING.md](./CONTRIBUTING.md)（待创建）。

提交格式：
```
feat(scope): add new feature
fix(scope): fix bug
docs(scope): update documentation
```

---

## 📄 License

MIT

---

## 🙏 致谢

- Go 社区
- React 社区
- gRPC 项目

---

**准备好开始了吗？** 🚀

```bash
./setup.sh && ./dev.sh
```
