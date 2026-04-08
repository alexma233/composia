# 简介

Composia 是一个围绕服务定义、单一控制平面和一个或多个执行代理构建的自托管服务管理器。

## 什么是 Composia？

Composia 让你能够：

- **管理 Docker Compose 服务** - 使用熟悉的 Docker Compose YAML 文件定义服务
- **多节点部署** - 将服务部署到多个节点（代理）上
- **集中控制** - 通过单一控制平面管理所有服务和节点
- **实时监控** - 查看服务状态、日志和资源使用情况

## 核心概念

### 服务定义（Service Definitions）

Composia 使用 `composia-meta.yaml` 文件来定义服务元数据，结合标准的 `docker-compose.yaml` 文件：

```yaml
# composia-meta.yaml
name: my-service
description: My awesome service
version: "1.0"
```

### 控制平面（Control Plane）

控制平面是 Composia 的大脑，负责：

- 管理服务定义和配置
- 协调代理节点
- 处理部署请求
- 聚合状态和指标

### 执行代理（Execution Agents）

代理运行在实际的 Docker 主机上，负责：

- 执行部署命令
- 监控容器状态
- 收集日志和指标
- 与控制平面通信

## 技术栈

- **后端**: Go 1.25+
- **前端**: SvelteKit + Bun
- **运行时**: Docker Compose
- **数据库**: SQLite
- **通信**: ConnectRPC

## 许可证

Composia 采用 AGPL-3.0 开源许可证发布。
