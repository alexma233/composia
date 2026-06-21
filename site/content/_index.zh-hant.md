---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>自由且開放原始碼</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  您的 Compose 檔案，無所不在
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  為進階使用者打造的自託管協調系統。以純文字定義服務、存放在 Git 中、所有設定都以檔案為基礎且無鎖定。
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="快速開始" link="docs" >}}
{{< hextra/hero-button text="存放庫" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
{{< hextra/hero-button text="為什麼選擇 Composia" link="docs/about/why-composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="多節點 Compose"
  subtitle="從簡單的設定檔將服務部署到任意節點。獨特的連線模式可穿透 NAT、防火牆與 CDN。"
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="標準檔案，無鎖定"
  subtitle="docker-compose.yaml + composia-meta.yaml，存放在您的 Git 存放庫中。開放格式，所有設定都以檔案為基礎，隨時手動控制。"
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="易用的 Web 儀表板"
  subtitle="檔案瀏覽與編輯、即時日誌、Docker 資源檢視與互動式終端機。支援行動裝置，提供從瀏覽器管理服務所需的一切功能。"
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI 與公開 API"
  subtitle="功能齊全的 CLI，適用於自動化腳本與 AI 代理。公開 API 讓第三方客戶端易於建置。"
  icon="terminal"
>}}

{{< hextra/feature-card
  title="備份與還原"
  subtitle="由 Rustic 驅動的自動化備份，支援排程執行、快照管理與隨需還原。"
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS 與反向代理"
  subtitle="Cloudflare DNS 管理與 Caddy 反向代理開箱即用。自動同步並重新載入您的 Caddyfile。"
  icon="globe"
>}}

{{< hextra/feature-card
  title="映像檔更新偵測"
  subtitle="自動偵測新的 Docker 映像檔標籤並套用更新。支援多種版本策略，可從 GitHub、Forgejo 等來源取得最新標籤。"
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="內建通知"
  subtitle="Email、Telegram 與 Alertmanager 通知，涵蓋任務結果、備份事件、映像檔更新與節點狀態變更。"
  icon="bell"
>}}

{{< hextra/feature-card
  title="還有更多…"
  icon="sparkles"
  subtitle="任務系統 / 加密密鑰 / 自動部署 / Prometheus 指標 / 跨平台支援 / 無障礙 / 以及更多…"
>}}

{{< /hextra/feature-grid >}}
