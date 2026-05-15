---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>自由开源</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Composia — 自托管的 Docker Compose 控制平面
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  用纯文本定义服务，一键部署到多节点，获得统一的基础设施可视化管理——无锁定，全掌控。
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="快速开始" link="docs" >}}
{{< hextra/hero-button text="Forgejo 仓库" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="多节点编排"
  subtitle="从一份 Git 仓库配置，部署服务到任意节点。Agent 拉取模式穿透 NAT 与防火墙。"
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="标准文件，无锁定"
  subtitle="docker-compose.yaml + composia-meta.yaml 存你自己的 Git 仓库。无专有格式，随时 SSH 直连任意节点。"
  icon="document"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="Web 面板"
  subtitle="实时服务状态、容器日志流、交互式终端、内置文件树与 YAML 编辑器。"
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI 与公共 API"
  subtitle="功能完备的终端客户端，助力自动化脚本。公开 ConnectRPC API，第三方客户端与 AI Agent 可直接调用。"
  icon="terminal"
>}}

{{< hextra/feature-card
  title="备份与恢复"
  subtitle="基于 Rustic 的自动备份，定时调度、快照管理、按需恢复。"
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS 与反向代理"
  subtitle="Cloudflare DNS 管理与 Caddy 反向代理开箱即用。自动同步并重载 Caddyfile。"
  icon="globe"
>}}

{{< /hextra/feature-grid >}}
