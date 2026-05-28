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
  你的 Compose 文件，无处不在
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  为进阶用户打造的自托管编排系统。&nbsp;<br class="hx:sm:block hx:hidden" />用纯文本定义服务，用 Git 管理，无需数据库、无平台绑定。&nbsp;<br class="hx:sm:block hx:hidden" />备份、DNS、反向代理和镜像更新——全部内置。
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="快速开始" link="docs" >}}
{{< hextra/hero-button text="仓库地址" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="多节点 Compose"
  subtitle="通过简单的配置将服务部署到任意节点。独有的连接模式可穿透 NAT、防火墙和 CDN。"
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="标准文件，无平台绑定"
  subtitle="docker-compose.yaml + composia-meta.yaml，存储在 Git 仓库中。开放格式，无数据库存储，随时可手动控制。"
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="易用的 Web 仪表盘"
  subtitle="文件浏览与编辑、实时日志、Docker 资源查看和交互式终端。支持移动端，满足从浏览器管理服务的所有需求。"
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI 与公共 API"
  subtitle="功能齐全的 CLI，可用于自动化脚本和 AI 智能体。公共 API 让第三方客户端易于构建。"
  icon="terminal"
>}}

{{< hextra/feature-card
  title="备份与恢复"
  subtitle="由 Rustic 驱动的自动备份，支持定时运行、快照管理和按需恢复。"
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS 与反向代理"
  subtitle="Cloudflare DNS 管理和 Caddy 反向代理开箱即用。自动同步并重载 Caddyfile。"
  icon="globe"
>}}

{{< hextra/feature-card
  title="镜像更新检测"
  subtitle="自动检测新的 Docker 镜像标签并应用更新。支持多种版本策略，可从 GitHub、Forgejo 等获取最新标签。"
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="内置通知"
  subtitle="邮件、Telegram 和 Alertmanager 通知，覆盖任务结果、备份事件、镜像更新和节点状态变化。"
  icon="bell"
>}}

{{< hextra/feature-card
  title="还有更多……"
  icon="sparkles"
  subtitle="任务系统 / 加密密钥 / 自动部署 / Prometheus 指标 / 跨平台支持 / 无障碍访问 / 更多……"
>}}

{{< /hextra/feature-grid >}}
