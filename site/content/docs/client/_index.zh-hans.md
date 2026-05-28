---
title: "客户端"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

与 Composia 交互的方式：

- **Web UI** — 基于浏览器的仪表盘，用于管理服务、节点、任务、备份和设置。启动堆栈后访问 `http://localhost:3000`。
- **CLI** — `composia` 命令行工具，用于脚本编写和终端工作流。通过[包管理器](/docs/installation/package-managers/)安装或从[发布页面](https://forgejo.alexma.top/alexma233/composia/releases)下载。

Web UI 和 CLI 都与同一个[控制器 API](/docs/developer-guide/api/) 通信。
