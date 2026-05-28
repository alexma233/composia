---
title: "リバースプロキシ"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia はリバースプロキシ管理のために Caddy と統合します。Caddy インフラストラクチャサービスは通常の Docker Compose サービスとして実行され、Composia はデプロイ時と停止時に Caddy 設定ファイルを同期します。

## アーキテクチャ

```
Controller repo
  ├── caddy/
  │   ├── docker-compose.yaml   (Caddy Compose サービス)
  │   ├── Caddyfile             (メイン Caddy 設定、生成ファイルをインポート)
  │   └── composia-meta.yaml    (infra.caddy を宣言)
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             (サービス固有の Caddy 設定)
  │   └── composia-meta.yaml    (network.caddy を宣言)
  └── ...
```

デプロイ時に、Composia は各サービスの Caddyfile を生成ディレクトリにコピーし、Caddy のリロードをトリガーします。

## インフラストラクチャのセットアップ

リポジトリに Caddy インフラストラクチャサービスを 1 つだけ宣言します:

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

Caddy サービスディレクトリのメイン Caddyfile は生成されたファイルをインポートする必要があります:

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| キー | 型 | 説明 |
|-----|------|-------------|
| `compose_service` | `string` | Compose サービス名。デフォルトは `caddy`。 |
| `config_dir` | `string` | コンテナ内の Caddy 設定ディレクトリ。デフォルトは `/etc/caddy`。 |

リポジトリ内で Caddy インフラストラクチャとして宣言できるサービスは 1 つだけです。

## サービス設定

リバースプロキシエントリが必要な各サービスについて、`composia-meta.yaml` で Caddy を有効にし、Caddyfile を提供します:

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

`source` パスはサービスディレクトリからの相対パスで、その中に留まる必要があります。ファイル名は任意ですが、`Caddyfile` が慣例です。

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## 同期の仕組み

デプロイまたは更新タスク中、エージェントは `compose up` の後に Caddy 同期ステップを実行します:

1. サービスの `composia-meta.yaml` から `network.caddy.source` を読み取ります。
2. ソースファイルを `<agent_state_dir>/caddy/generated/<service_dir>.caddy` にコピーします。
3. `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile` を実行します。

生成されるファイル名はサービスディレクトリ名から派生します。`my-app` の場合、ファイルは `my-app.caddy` になります。

停止タスク中は、生成された Caddy ファイルが削除されます。

## Caddy 同期タスク

スタンドアロンの `caddy_sync` タスクはサービスをデプロイせずに Caddy 設定を再構築します。2 つのモードで動作します:

**完全再構築**（`full_rebuild: true`）: 生成ディレクトリからすべての `.caddy` ファイルを削除し、すべての Caddy 管理サービスを再同期します。

**ターゲット同期**: 指定されたサービスディレクトリのみを同期します。

Web UI または CLI からトリガーします:

```bash
composia service caddy-sync my-app
```

## Caddy リロードタスク

`caddy_reload` タスクはファイルを変更せずに Caddy コンテナ内で `caddy reload` を実行します。メイン Caddyfile を手動で編集した後に使用します:

```bash
composia node reload-caddy main
```

## エージェント設定

エージェント設定にはオプションの Caddy セクションがあります:

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| キー | 型 | 説明 |
|-----|------|-------------|
| `generated_dir` | `string` | 生成された Caddy 設定ディレクトリ。デフォルトは `<state_dir>/caddy/generated`。 |

生成ディレクトリは Caddy コンテナが読み取れるパス内にある必要があります。Caddy compose サービスは、このディレクトリをメイン Caddyfile でインポートされるパスにマウントするボリュームを持つ必要があります。
