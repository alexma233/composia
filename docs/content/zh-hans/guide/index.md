# 简介

Composia 是一个围绕服务定义、单一控制平面和多个执行代理构建的自托管 Docker Compose 管理平台。

## 什么是 Composia？

Composia 让你能够：

- **管理 Docker Compose 服务** —— 使用 `docker-compose.yaml` 配合少量 `composia-meta.yaml` 元数据定义服务
- **多节点部署** —— 将服务部署到多个节点（代理）上
- **集中控制** —— 通过单一控制平面管理所有服务和节点
- **运行态可见性** —— 查看服务状态、任务日志、节点摘要，以及按节点划分的 Docker 详情

## 适用场景

- 个人或团队需要管理多个 Docker 服务
- 需要在多台服务器上部署和协调服务
- 希望通过 Web 界面直观管理容器化应用
- 需要自动化备份、DNS 管理等运维能力

## 核心概念

### 服务定义（Service Definitions）

Composia 使用 `composia-meta.yaml` 定义服务元数据，结合标准的 `docker-compose.yaml` 文件：

```yaml
# composia-meta.yaml
name: my-service
nodes:
  - main
```

### 控制平面（Control Plane）

控制平面是 Composia 的核心，负责：

- 管理服务定义和配置
- 协调代理节点
- 处理部署请求
- 聚合状态、任务和 Docker 摘要

### 执行代理（Execution Agents）

代理运行在实际的 Docker 主机上，负责：

- 执行部署命令
- 上报心跳、任务结果、日志和 Docker 摘要
- 与控制平面通信

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.25+ |
| WebUI | SvelteKit + Bun |
| 运行时 | Docker Compose |
| 数据库 | SQLite |
| 通信 | ConnectRPC |

## 文档导航

- [为什么选择 Composia？](./why-composia) —— 理解项目的核心宗旨，以及它和其他方案的区别
- [快速开始](./quick-start) —— 几分钟内启动并运行
- [核心概念](./core-concepts) —— 理解 Composia 的工作原理
- [架构概览](./architecture) —— 系统架构详解
- [配置指南](./configuration) —— 平台和服务配置说明
- [服务定义](./service-definition) —— 如何定义和管理服务
- [部署管理](./deployment) —— 部署、更新、停止和重启
- [DNS 配置](./dns) —— 服务侧 DNS 配置
- [Caddy 配置](./caddy) —— Caddy 反向代理配置
- [备份与迁移](./backup-migrate) —— 数据保护和迁移策略
- [日常运维](./operations) —— 任务系统和资源管理
- [开发指南](./development) —— 本地开发环境搭建
- [API 参考](./api/) —— 基于 protobuf 注释生成的 RPC 参考文档

## 许可证

Composia 采用 [AGPL-3.0](https://www.gnu.org/licenses/agpl-3.0.html) 开源许可证发布。
