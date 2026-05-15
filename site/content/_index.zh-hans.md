---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>自由且开源</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  你的 Compose 文件，无处不在
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  为高阶用户精心打造的自托管编排系统。&nbsp;<br class="hx:sm:block hx:hidden" />用纯文本定义服务，放在 Git 仓库，无数据库，无锁定。&nbsp;<br class="hx:sm:block hx:hidden" />备份、DNS、反向代理、镜像更新都一应俱全。
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="快速开始" link="docs" >}}
{{< hextra/hero-button text="代码仓库" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="多节点 Compose"
  subtitle="从一份简单的配置，部署服务到任意节点。独特的连接模式穿透 NAT、防火墙与 CDN。"
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="标准文件，无锁定"
  subtitle="docker-compose.yaml + composia-meta.yaml，保存在你的 Git 仓库。无专有格式和数据库，随时手动操作。"
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="易用的 Web 面板"
  subtitle="文件浏览和编辑、实时日志、Docker 资源查看、交互式终端。移动端友好，用浏览器管理服务所需的一切。"
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI 与公共 API"
  subtitle="功能完备的 CLI，为自动化脚本和 AI Agent 准备好。公开 API，助力第三方客户端。"
  icon="terminal"
>}}

{{< hextra/feature-card
  title="备份与恢复"
  subtitle="基于 Rustic 的自动备份，定时调度、快照管理、按需恢复。将安全放在第一位。"
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS 与反向代理"
  subtitle="Cloudflare DNS 管理与 Caddy 反向代理开箱即用。自动同步并重载 Caddyfile。"
  icon="globe"
>}}

{{< hextra/feature-card
  title="镜像更新检测"
  subtitle="自动检测 Docker 镜像新 Tag，并应用更新。支持各种版本策略，也可从 GitHub、Forgejo 等平台获取最新 Tag。"
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="内置通知"
  subtitle="邮件 / Telegram / Alertmanager 通知，覆盖任务结果、备份事件、镜像更新、节点上下线。"
  icon="bell"
>}}

{{< hextra/feature-card
  title="还有更多…"
  icon="sparkles"
  subtitle="任务系统 / 加密 Secrets / 自动部署 / Prometheus metrics / 全平台支持 / 无障碍 / 等等…"
>}}

{{< /hextra/feature-grid >}}
