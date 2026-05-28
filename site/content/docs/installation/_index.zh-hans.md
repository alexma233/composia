---
title: "安装"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia 有四个运行时二进制文件和镜像：

| 组件 | 用途 |
|-----------|---------|
| `composia-controller` | 运行 API、任务队列、期望状态 Git 仓库以及控制器端集成。 |
| `composia-agent` | 在每个 Docker 节点上运行，执行 Docker Compose 操作。 |
| `composia-web` | 与控制器通信的浏览器 UI。 |
| `composia` | 用于终端、脚本和自动化的 CLI。 |

## 选择安装方式

| 方式 | 适用场景 |
|--------|----------|
| [Docker Compose](docker-compose/) | 快速一体化部署，包含控制器、本地 agent 和 Web UI。 |
| [包管理器与二进制文件](package-managers/) | 非容器安装、操作系统包、Nix、AUR 和手动下载归档。 |
| [配置](configuration/) | 配置文件、Web 环境变量、age 密钥设置和完整的全局配置参考。 |

对于源码构建，请参阅[开发者指南：源码构建](/docs/developer-guide/source-build/)。
