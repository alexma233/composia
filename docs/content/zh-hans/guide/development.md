# 开发指南

本指南介绍如何为 Composia 搭建本地开发环境，以及项目结构和贡献规范。

## 前提条件

| 工具 | 版本要求 | 说明 |
|------|----------|------|
| Go | 1.25+ | 后端开发语言 |
| Bun | 1.3+ | 前端包管理器和运行时 |
| Docker | 20.10+ | 容器运行时 |
| Docker Compose | v2.0+ | 容器编排 |
| SQLite3 | 3.35+ | 数据库 |
| Git | 2.30+ | 版本控制 |
| buf | 1.30+ | Protobuf 代码生成 |

## 环境搭建

### 1. 克隆仓库

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. 安装前端依赖

```bash
cd web
bun install
cd ..
```

### 3. 初始化开发配置

```bash
# 创建必要的目录
mkdir -p repo-controller repo-agent

# 初始化 Git 仓库（Controller 需要）
git init repo-controller
```

## 启动开发环境

### 方式一：分别启动前后端

**启动前端开发服务器：**

```bash
cd web
bun run dev
```

前端将在 `http://localhost:5173` 运行。

**启动 Controller（终端 2）：**

```bash
go run ./cmd/composia controller \
  -config ./configs/config.controller.dev.yaml
```

**启动 Agent（终端 3）：**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent.dev.yaml
```

### 方式二：使用 Docker Compose

```bash
docker compose up -d
```

## 开发配置示例

### Controller 配置

```yaml
# configs/config.controller.dev.yaml
controller:
  listen_addr: "127.0.0.1:7001"
  controller_addr: "http://127.0.0.1:7001"
  repo_dir: "./repo-controller"
  state_dir: "./state-controller"
  log_dir: "./logs"
  cli_tokens:
    - name: "dev-admin"
      token: "dev-admin-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
```

### Agent 配置

```yaml
# configs/config.agent.dev.yaml
agent:
  controller_addr: "http://127.0.0.1:7001"
  node_id: "node-2"
  token: "node-2-token"
  repo_dir: "./repo-agent-node-2"
  state_dir: "./state-agent-node-2"
```

## 测试多节点场景

启动多个 Agent 模拟多节点环境：

**Agent 1：**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.controller.dev.yaml
```

**Agent 2：**

```bash
go run ./cmd/composia agent \
  -config ./configs/config.agent.dev.yaml
```

## 代码生成

### 生成 Protobuf 代码

修改 `.proto` 文件后，重新生成 Go 代码：

```bash
buf generate
```

## 项目结构

```
composia/
├── cmd/
│   └── composia/           # 主程序入口
│       └── main.go
├── configs/                # 开发配置示例
├── docs/                   # 文档（VitePress）
│   ├── content/
│   └── .vitepress/
├── gen/
│   └── go/                 # 生成的 protobuf 代码
├── internal/               # 内部包
│   ├── controller/         # Controller 实现
│   ├── agent/              # Agent 实现
│   ├── repo/               # 服务仓库解析与校验
│   ├── store/              # 基于 SQLite 的状态存储
│   └── ...
├── proto/                  # Protobuf 源文件
├── web/                    # SvelteKit 前端
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/ # UI 组件
│   │   │   └── server/     # 服务端 Controller 访问层
│   │   └── routes/         # 页面路由
│   └── package.json
├── docker-compose.yaml     # 本地/接近生产的 Compose 栈
└── README.md
```

## 关键目录说明

| 目录 | 说明 |
|------|------|
| `internal/controller/` | Controller 业务逻辑 |
| `internal/agent/` | Agent 业务逻辑 |
| `proto/` | Protobuf 源定义 |
| `internal/store/` | 数据存储层 |
| `web/src/lib/server/` | 服务端 Controller 访问 |
| `web/src/lib/components/` | 可复用 UI 组件 |

## 代码规范

### Go 代码

- 遵循 [Effective Go](https://go.dev/doc/effective_go)
- 使用 `gofmt` 格式化代码
- 使用 `golint` 检查代码风格
- 重要函数添加注释

### 前端代码

- 使用 TypeScript 严格模式
- 遵循 Svelte 5 语法（使用 Runes）
- 组件使用 `$props()` 声明属性
- 使用 `shadcn-svelte` UI 组件库

## 测试

### 运行后端测试

```bash
go test ./...
```

### 运行前端检查

```bash
bun run web:check
```

## 调试技巧

### Controller 调试

```bash
# 使用明确的配置文件启动 Controller
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### Agent 调试

```bash
go run ./cmd/composia agent -config ./configs/config.controller.dev.yaml
```

### 查看 RPC 通信

当前 Controller 没有注册 gRPC reflection。

如需排查 RPC，请优先使用 Web 端生成的 Connect 客户端，或直接调用已注册的 ConnectRPC 方法。

## 提交代码

1. 确保代码通过测试
2. 遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范
3. 提交前运行代码格式化

```bash
# 格式化 Go 代码
gofmt -w .

# 格式化前端代码
cd web && bun run format
```

## 常见问题

**Q: Controller 启动报错 "repo not initialized"**

A: 需要先初始化 Git 仓库：`git init repo-controller`

**Q: Agent 连接失败**

A: 检查 Controller 地址和 Token 是否匹配

**Q: 前端请求失败**

A: 确保 Controller 已启动，并检查 Web 进程的 `COMPOSIA_CONTROLLER_ADDR` 和 `COMPOSIA_CLI_TOKEN` 是否配置正确
