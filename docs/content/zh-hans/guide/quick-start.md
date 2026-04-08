# 快速开始

本指南将帮助你在几分钟内启动并运行 Composia。

## 前提条件

在开始之前，请确保已安装以下工具：

- Go 1.25+
- Bun 1.3+
- Docker Engine + Docker Compose v2
- SQLite3
- Git

## 安装

### 1. 克隆仓库

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. 安装前端依赖

```bash
bun install
```

### 3. 初始化控制器仓库

```bash
mkdir -p ./repo-controller && git -C ./repo-controller init
```

## 启动服务

### 启动控制平面

```bash
go run ./cmd/composia controller -config ./configs/config.controller.dev.yaml
```

### 启动代理

在另一个终端中：

```bash
go run ./cmd/composia agent -config ./configs/config.agent.dev.yaml
```

### 启动前端开发服务器

```bash
bun run dev
```

## 访问界面

打开浏览器访问 http://localhost:5173 查看 Web 界面。

## 下一步

- 了解 [架构](./architecture) 详情
- 查看 API 文档
- 部署你的第一个服务
