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

仓库现在提供了 `mise.toml` 来管理 `go`、`bun` 和 `buf`。

如果你使用 `mise`，安装并激活 `mise` 后先执行：

```bash
mise install
```

`docker`、`docker compose`、`git` 和 `sqlite3` 仍建议使用系统安装。

## 环境搭建

### 1. 克隆仓库

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. 初始化开发配置

```bash
# 创建必要的目录
mkdir -p dev/repo-controller dev/repo-agent

# 初始化 Git 仓库（Controller 需要）
git init dev/repo-controller
```

## 启动开发环境

### 方式一：全容器化开发 + 自动重载（推荐）

```bash
mise run dev
```

这套开发栈会启动：

- `controller-dev`：`http://localhost:7001`
- `web-dev`：`http://localhost:5173`
- `docs-dev`：`http://localhost:5174`
- `agent-dev`：连接本地 Docker socket

走这条路径时，你不需要先在宿主机执行 `bun install`。`web-dev` 和 `docs-dev` 会在容器启动时自行安装工作区依赖。

它默认直接复用 `dev/` 下现有的开发状态目录：

- `./dev/repo-controller`
- `./dev/state-controller`
- `./dev/repo-agent`
- `./dev/state-agent`
- `./dev/logs`

因此你之前手动启动 Controller 或 Agent 时留下的服务定义、SQLite 数据库和任务日志，会直接带进容器开发栈。

自动重载行为：

- `web-dev` 使用 `vite dev`
- `docs-dev` 使用 `vitepress dev`
- `controller-dev` 和 `agent-dev` 使用 `air` 监听 Go 源码变化

代码通过 bind mount 挂载进容器，修改后会自动生效。

停止开发栈：

```bash
mise run dev:down
```

查看开发栈日志：

```bash
mise run dev:logs
```

如果宿主机启用了 SELinux，请改用带 override 的入口：

```bash
mise run dev:selinux
```

它会额外加载 `dev/docker-compose.dev.selinux.override.yaml`，为开发容器设置 `label=disable`，避免源码目录 bind mount 在 SELinux 主机上被拒绝访问。

停止这套栈：

```bash
mise run dev:down:selinux
```

查看这套栈的日志：

```bash
mise run dev:logs:selinux
```

### 方式二：本机工具链开发

先在宿主机安装工作区依赖：

```bash
bun install
```

**启动前端开发服务器：**

```bash
mise run web
```

前端将在 `http://localhost:5173` 运行。

如需启动文档开发服务器：

```bash
mise run docs
```

文档站默认运行在 `http://localhost:5174`。

**启动 Controller（终端 2）：**

```bash
mise run controller
```

**启动 Agent（终端 3）：**

```bash
mise run agent
```

这里的 `agent` 会使用共享的 `configs/config.controller.dev.yaml`，以 `main` 节点身份接入本地 Controller。

如果你要额外启动一个 `node-2` 本地 Agent，可以执行：

```bash
mise run agent2
```

### 方式三：预构建镜像栈

```bash
docker compose up -d
```

这套 Compose 栈使用的是预构建镜像，适合做集成联调或接近生产的本地验证，不提供源码热更新，也不会在你修改代码后自动重新 build。

## 开发配置示例

### Controller 配置

```yaml
# configs/config.controller.dev.yaml
controller:
  listen_addr: "127.0.0.1:7001"
  controller_addr: "http://127.0.0.1:7001"
  repo_dir: "./dev/repo-controller"
  state_dir: "./dev/state-controller"
  log_dir: "./dev/logs"
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
  repo_dir: "./dev/repo-agent-node-2"
  state_dir: "./dev/state-agent-node-2"
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

A: 需要先初始化 Git 仓库：`git init dev/repo-controller`

**Q: Agent 连接失败**

A: 检查 Controller 地址和 Token 是否匹配

**Q: 前端请求失败**

A: 确保 Controller 已启动，并检查 Web 进程的 `COMPOSIA_CONTROLLER_ADDR` 和 `COMPOSIA_CLI_TOKEN` 是否配置正确
