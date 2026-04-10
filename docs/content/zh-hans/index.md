---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose 控制平面"
  tagline: 一个平台无关的 Docker Compose 控制平面，在不牺牲纯文件和 CLI 工作流的前提下统一管理多节点自托管基础设施
  image:
    src: /logo.svg
    alt: Composia
  actions:
    - theme: brand
      text: 快速开始
      link: /zh-hans/guide/quick-start
    - theme: alt
      text: 为什么选择 Composia？
      link: /zh-hans/guide/why-composia
    - theme: alt
      text: 查看 Forgejo
      link: https://forgejo.alexma.top/alexma233/composia

features:
  - title: 🐳 原生 Docker Compose
    details: 保持 Docker Compose 和纯文件工作流为核心，而不是把配置锁进平台私有模型。
  - title: 🎛️ 单一控制平面
    details: 通过单一控制平面统一协调服务、节点、任务和状态，同时保留对底层系统的直接控制。
  - title: 🤖 多代理架构
    details: 支持一个或多个执行代理，可横向扩展以管理大规模基础设施。
  - title: 📋 运行态可见性
    details: 统一查看服务状态、任务日志、节点摘要，以及磁盘容量和 Docker 资源统计。
  - title: 🧾 文件优先
    details: 期望状态保存在仓库和普通文件里，便于审查、迁移和继续使用标准 CLI 工具。
  - title: 🔒 安全可靠
    details: 采用 AGPL-3.0 开源协议，代码透明可审计，支持私有部署。
  - title: 🚫 避免平台锁定
    details: 控制平面负责协调、校验和汇总，而不是夺走操作者对基础设施本身的主权。
---
