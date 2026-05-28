---
title: "インストール"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Composia には 4 つのランタイムバイナリとイメージがあります:

| コンポーネント | 目的 |
|-----------|---------|
| `composia-controller` | API、タスクキュー、期待状態 Git リポジトリ、コントローラー側の統合を実行します。 |
| `composia-agent` | 各 Docker ノード上で実行され、Docker Compose 操作を実行します。 |
| `composia-web` | コントローラーと通信するブラウザ UI。 |
| `composia` | ターミナル、スクリプト、自動化のための CLI。 |

## 方法の選択

| 方法 | 最適な用途 |
|--------|----------|
| [Docker Compose](docker-compose/) | コントローラー、ローカルエージェント、Web UI を含む高速なオールインワンデプロイ。 |
| [パッケージマネージャーとバイナリ](package-managers/) | 非コンテナインストール、OS パッケージ、Nix、AUR、手動アーカイブ。 |
| [設定](configuration/) | 設定ファイル、Web 環境変数、age 鍵のセットアップ、完全なグローバル設定リファレンス。 |

ソースビルドについては [開発者ガイド: ソースビルド](/docs/developer-guide/source-build/) を参照してください。
