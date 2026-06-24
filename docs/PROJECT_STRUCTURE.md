# Polyglot 项目结构说明

## 📁 目录结构

```
new/polyglot/
├── src/
│   ├── srv/                    # 后端服务（Go）
│   │   ├── cmd/polyglot/       # 主程序入口
│   │   ├── internal/           # 私有代码
│   │   │   ├── adapter/        # Adapter 接口和注册中心
│   │   │   ├── config/         # 配置管理
│   │   │   ├── server/         # HTTP 服务器
│   │   │   │   ├── handler/    # 请求处理器
│   │   │   │   └── middleware/ # 中间件
│   │   │   └── store/          # 数据存储
│   │   ├── pkg/                # 公共库
│   │   │   ├── logger/         # 日志
│   │   │   └── errors/         # 错误定义
│   │   ├── proto/              # gRPC 协议定义
│   │   │   ├── adapter/
│   │   │   │   └── adapter.proto
│   │   │   └── generate.sh     # 代码生成脚本
│   │   ├── configs/            # 配置文件
│   │   │   └── config.yaml
│   │   ├── go.mod
│   │   └── Makefile
│   │
│   └── web/                    # 前端（React）
│       ├── src/
│       │   ├── pages/          # 页面组件
│       │   │   ├── Dashboard.tsx
│       │   │   ├── Adapters.tsx
│       │   │   ├── Logs.tsx
│       │   │   └── Settings.tsx
│       │   ├── components/     # 复用组件
│       │   │   └── Layout.tsx
│       │   ├── api/            # API 客户端
│       │   │   └── client.ts
│       │   ├── styles/         # 样式文件
│       │   ├── App.tsx
│       │   └── main.tsx
│       ├── public/             # 静态资源
│       ├── index.html
│       ├── package.json
│       ├── tsconfig.json
│       └── vite.config.ts
│
└── README.md
```

---

## 🚀 快速开始

### 1. 生成 gRPC 代码

```bash
cd src/srv/proto
./generate.sh
```

### 2. 启动后端

```bash
cd src/srv
go mod tidy
go run cmd/polyglot/main.go
```

### 3. 启动前端

```bash
cd src/web
npm install
npm run dev
```

访问 http://localhost:3001/ui/

---

## 📝 项目特性

- ✅ 前后端分离架构
- ✅ gRPC 协议定义
- ✅ React + TypeScript 前端
- ✅ Go 后端框架
- ✅ 清晰的目录结构

更多文档请查看 `migration/` 目录。
