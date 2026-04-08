# 开发

本指南介绍如何为 Composia 搭建本地开发环境。

## 前提条件

- Go 1.25+
- Bun 1.3+
- Docker Engine + Docker Compose v2
- SQLite3
- Git

## 克隆仓库

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

## 安装依赖

### 前端

```bash
bun install
```

### 后端

Go 依赖通过 `go mod` 自动管理。

## 启动开发服务器

### 前端开发服务器

```bash
bun run dev
```

Web 界面将在 `http://localhost:5173` 可用。

### 后端 - 控制器

首先初始化控制器仓库：

```bash
mkdir -p ./repo-controller && git -C ./repo-controller init
```

然后启动控制器：

```bash
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### 后端 - 代理

在另一个终端中启动代理：

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

## 运行第二个代理

要测试多节点场景，请使用不同的节点 ID 运行第二个代理：

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

## 生成 Protobuf 存根

修改 `.proto` 文件后，重新生成 Go 代码：

```bash
buf generate
```

## 项目结构

```text
cmd/composia/         # composia 入口
configs/              # 本地开发配置示例
gen/go/               # 生成的 protobuf 和 Connect 代码
internal/             # 后端包
proto/                # protobuf 定义
web/                  # SvelteKit 前端
```

## 贡献

请确保代码遵循现有模式，并包含适当的测试。
