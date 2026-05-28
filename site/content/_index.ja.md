---
title: Composia
layout: hextra-home
---

{{< hextra/hero-badge link="https://forgejo.alexma.top/alexma233/composia" >}}
  <div class="hx:w-2 hx:h-2 hx:rounded-full hx:bg-primary-400"></div>
  <span>フリー＆オープンソース</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  あなたの Compose ファイルを、どこでも
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  パワーユーザーのために作られたセルフホスト型オーケストレーションシステム。&nbsp;<br class="hx:sm:block hx:hidden" />サービスをプレーンテキストで定義し、Git で管理、データベース不要・ロックイン不要。&nbsp;<br class="hx:sm:block hx:hidden" />バックアップ、DNS、リバースプロキシ、イメージ更新 — すべて標準装備。
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="クイックスタート" link="docs" >}}
{{< hextra/hero-button text="リポジトリ" link="https://forgejo.alexma.top/alexma233/composia" style="background-color: transparent; border: 1px solid var(--tw-prose-headings); color: var(--tw-prose-headings);" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}

{{< hextra/feature-card
  title="マルチノード Compose"
  subtitle="シンプルな設定で任意のノードにサービスをデプロイ。独自の接続モードが NAT、ファイアウォール、CDN を越えて動作します。"
  icon="server"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(236,72,153,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="標準ファイル、ロックインなし"
  subtitle="Git リポジトリに保存する docker-compose.yaml + composia-meta.yaml。オープンフォーマット、データベースフリーストレージ、いつでも手動操作可能。"
  icon="lock-open"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(16,185,129,0.1),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="使いやすい Web ダッシュボード"
  subtitle="ファイルの閲覧と編集、ライブログ、Docker リソースビュー、インタラクティブターミナル。モバイル対応、ブラウザからすべてのサービス管理が可能。"
  icon="desktop-computer"
  style="background: radial-gradient(ellipse at 50% 80%,rgba(37,99,235,0.12),hsla(0,0%,100%,0));"
>}}

{{< hextra/feature-card
  title="CLI とパブリック API"
  subtitle="自動化スクリプトや AI エージェントに対応したフル機能の CLI。公開 API によりサードパーティクライアントの構築も容易。"
  icon="terminal"
>}}

{{< hextra/feature-card
  title="バックアップとリストア"
  subtitle="Rustic による自動バックアップ。スケジュール実行、スナップショット管理、オンデマンドリストアに対応。"
  icon="save"
>}}

{{< hextra/feature-card
  title="DNS とリバースプロキシ"
  subtitle="Cloudflare DNS 管理と Caddy リバースプロキシがそのまま動作。Caddyfile を自動同期・リロード。"
  icon="globe"
>}}

{{< hextra/feature-card
  title="イメージ更新検出"
  subtitle="新しい Docker イメージタグを自動検出して更新を適用。複数のバージョニング戦略に対応し、GitHub、Forgejo などから最新タグを取得可能。"
  icon="arrow-circle-up"
>}}

{{< hextra/feature-card
  title="ビルトイン通知"
  subtitle="タスク結果、バックアップイベント、イメージ更新、ノード状態変化を Email、Telegram、Alertmanager で通知。"
  icon="bell"
>}}

{{< hextra/feature-card
  title="その他…"
  icon="sparkles"
  subtitle="タスクシステム / 暗号化シークレット / 自動デプロイ / Prometheus メトリクス / クロスプラットフォーム対応 / アクセシビリティ / その他…"
>}}

{{< /hextra/feature-grid >}}
