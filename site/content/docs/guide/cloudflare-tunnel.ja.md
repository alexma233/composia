---
title: "Cloudflare Tunnel"
date: '2026-05-31T00:00:00+08:00'
weight: 25
---

Composia は、`network.cloudflare_tunnel` を宣言するサービスのリモート設定された Cloudflare Tunnel のイングレスルールを管理できます。トンネル同期はコントローラー側のタスクとして実行されます。これは、Cloudflare のリモートトンネル設定がグローバルな状態であるためです。

## 仕組み

サービスがデプロイ、更新、停止、または手動で同期されると、コントローラーは `cloudflare_tunnel_sync` タスクを作成します。コントローラーワーカーがこれを実行します：

1. タスクのリポジトリリビジョンですべてのサービスメタデータを読み取ります。
2. `network.cloudflare_tunnel` を宣言するサービスからトンネルイングレスルールを構築します。
3. 完全なイングレスリストを Cloudflare に `PUT /accounts/{account_id}/cfd_tunnel/{tunnel_id}/configurations` で送信します。
4. 各ホスト名に `{tunnel_id}.cfargotunnel.com` を指すプロキシされた CNAME があることを確認します。

Cloudflare はキャッチオールのイングレスルールを必要とします。Composia はデフォルトで `http_status:404` を追加します。

## コントローラー設定

トンネル ID と Cloudflare の認証情報は、サービスのメタデータではなく、コントローラー設定に属します：

```yaml
controller:
  cloudflare_tunnel:
    account_id: "REPLACE"
    api_token_file: /run/secrets/cloudflare-api-token
    tunnels:
      edge:
        tunnel_id: "c1744f8b-faa1-48a4-9e5c-02ac921467fa"
        fallback_service: http_status:404
    nodes:
      main:
        tunnel: edge
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `account_id` | `string` | はい | Cloudflare アカウント ID。 |
| `api_token` / `api_token_file` | `string` | はい | Cloudflare Tunnel 設定および DNS 書き込み権限を持つ API トークン。 |
| `tunnels` | `map` | はい | Cloudflare トンネル ID にマッピングされたトンネルエイリアス。 |
| `nodes` | `map` | いいえ | サービスが `tunnel` を指定しない場合に使用されるデフォルトのノードからトンネルへのマッピング。 |

`tunnel_id` はコネクターシークレットではありませんが、コントローラーレベルのインフラストラクチャメタデータです。cloudflared コネクタートークンまたは認証情報は、`cloudflared` サービスが使用するノード/エージェントのシークレットに保持する必要があります。

## サービス宣言

サービスの `composia-meta.yaml` でトンネルイングレスを宣言します：

```yaml
network:
  cloudflare_tunnel:
    hostname: app.example.com
    service: http://app:8080
    origin_request:
      no_tls_verify: false
      http_host_header: app.internal
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `hostname` | `string` | はい | Cloudflare Tunnel によってルーティングされる公開ホスト名。 |
| `service` | `string` | はい | `cloudflared` が使用するオリジン URL（例: `http://app:8080`）。 |
| `tunnel` | `string` | いいえ | トンネルエイリアス。省略された場合、Composia はターゲットノードのマッピングから導出します。 |
| `path` | `string` | いいえ | イングレスルールのオプションのパスマッチャー。 |
| `origin_request` | `object` | いいえ | Cloudflare オリジンパラメータ。初期サポートには `no_tls_verify`、`http_host_header`、`origin_server_name`、`connect_timeout`、`tls_timeout` が含まれます。 |

## トンネルの選択

Composia は以下のルールで各サービスのトンネルを解決します：

1. `network.cloudflare_tunnel.tunnel` が設定されている場合、そのエイリアスが使用されます。
2. サービスが 1 つのノードをターゲットとする場合、Composia は `controller.cloudflare_tunnel.nodes.<node>.tunnel` を使用します。
3. サービスが複数のノードをターゲットとし、すべてのノードが同じトンネルにマッピングされる場合、そのトンネルが使用されます。
4. ターゲットノードが異なるトンネルにマッピングされる場合、サービスは `network.cloudflare_tunnel.tunnel` を明示的に設定する必要があります。

## 停止時の動作

停止されたサービスが `network.cloudflare_tunnel` を宣言していた場合、後続のトンネル同期はそのサービスを除外し、その CNAME を削除します。以降の同期には実行中のインスタンスがあるサービスのみが含まれるため、サービスを再度デプロイすると再追加されます。

## 手動同期

CLI を使用してサービスのトンネル設定を同期します：

```bash
composia service my-app tunnel-sync
```

これは、選択されたサービスだけでなく、完全に設定されたトンネル状態を同期します。これは、Cloudflare のリモートトンネル設定が 1 つのドキュメントとして更新されるためです。
