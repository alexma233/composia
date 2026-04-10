---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose 控制平面"
  tagline: 面向自托管场景的服务管理平台，使用服务定义和执行代理统一管理多节点 Docker Compose 部署
  image:
    src: /logo.svg
    alt: Composia
  actions:
    - theme: brand
      text: 快速开始
      link: /zh-hans/guide/quick-start
    - theme: alt
      text: 查看 Forgejo
      link: https://forgejo.alexma.top/alexma233/composia

features:
  - title: 🐳 原生 Docker Compose
    details: 以 Docker Compose 为基础，只额外引入一个用于描述元数据的 `composia-meta.yaml`。
  - title: 🎛️ 单一控制平面
    details: 集中式控制平面管理所有服务和节点，提供统一的视图和控制接口。
  - title: 🤖 多代理架构
    details: 支持一个或多个执行代理，可横向扩展以管理大规模基础设施。
  - title: 📋 运行态可见性
    details: 统一查看服务状态、任务日志、节点摘要，以及磁盘容量和 Docker 资源统计。
  - title: 🔒 安全可靠
    details: 采用 AGPL-3.0 开源协议，代码透明可审计，支持私有部署。
  - title: ⚡ 现代技术栈
    details: Go 后端 + SvelteKit WebUI，围绕 ConnectRPC、SQLite 和 Docker Compose 构建。
---
