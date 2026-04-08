---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Composia"
  text: "Docker Compose 服务管理平台"
  tagline: 基于服务定义、单一控制平面和多代理架构的自托管服务管理器
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
    details: 完全兼容 Docker Compose，无需学习新的配置格式，使用熟悉的 YAML 定义服务。
  - title: 🎛️ 单一控制平面
    details: 集中式控制平面管理所有服务和节点，提供统一的视图和控制接口。
  - title: 🤖 多代理架构
    details: 支持一个或多个执行代理，可横向扩展以管理大规模基础设施。
  - title: 📊 内置监控
    details: 实时监控服务状态、资源使用情况和日志，快速发现和解决问题。
  - title: 🔒 安全可靠
    details: 采用 AGPL-3.0 开源协议，代码透明可审计，支持私有部署。
  - title: ⚡ 现代技术栈
    details: Go 后端 + SvelteKit 前端，高性能、低资源占用，响应迅速。
---
