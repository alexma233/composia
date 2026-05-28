---
title: "DNS"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

Composia は `network.dns` を宣言したサービスの DNS レコードを管理します。DNS 更新はコントローラー側のタスクとして実行されます。

## 動作の仕組み

サービスがデプロイされるか、DNS 更新が手動でトリガーされると、コントローラーは `dns_update` タスクを作成します。コントローラーワーカーがそれを実行します:

1. タスクに記録されたリポジトリリビジョンでサービスメタを読み取ります。
2. `network.dns` から期待する DNS レコードを構築します。
3. レコードを DNS プロバイダーに同期します。

## プロバイダー設定

コントローラー設定で少なくとも 1 つの DNS プロバイダーを設定します。プロバイダーの認証情報とゾーンリストはグローバルです:

```yaml
controller:
  dns:
    cloudflare:
      api_token: "REPLACE"
      zones:
        - "example.com"
        - "other.com"
```

5 つのプロバイダーがサポートされています。それぞれ固有の認証情報キーを持ち、管理対象ドメインゾーンをリストする `zones` フィールドを共有します:

| プロバイダー | キープレフィックス | 認証情報キー |
|----------|-----------|-----------------|
| `cloudflare` | `dns.cloudflare` | `api_token`, `api_token_file` |
| `alidns` | `dns.alidns` | `access_key_id`, `access_key_secret`, `region_id`, オプション `security_token` |
| `dnspod` | `dns.dnspod` | `secret_id`, `secret_key`, `region`, オプション `session_token` |
| `route53` | `dns.route53` | `access_key_id`, `secret_access_key`, `region`, オプション `session_token`, `profile`, `hosted_zone_id` |
| `huaweicloud` | `dns.huaweicloud` | `access_key_id`, `secret_access_key`, `region_id` |

各認証情報フィールドには、ファイルから読み取るための対応する `_file` バリアントがあります（例: `api_token_file`）。

## サービス DNS 宣言

サービスの `composia-meta.yaml` で DNS 設定を宣言します:

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: "Managed by Composia"
```

| キー | 型 | 必須 | 説明 |
|-----|------|----------|-------------|
| `provider` | `string` | はい | `cloudflare`、`alidns`、`dnspod`、`route53`、`huaweicloud` のいずれか。 |
| `hostname` | `string` | はい | DNS ホスト名。ゾーンは設定されたゾーンリストからマッチングされます。 |
| `record_type` | `string` | いいえ | `A`、`AAAA`、`CNAME`。空の場合、レコードタイプは値またはノードアドレスから推論されます。 |
| `value` | `string` | いいえ | 明示的な DNS レコード値。空の場合、Composia はターゲットノードから値を導出します。 |
| `proxied` | `bool` | いいえ | Cloudflare プロキシを有効にします。Cloudflare のみ対応。 |
| `ttl` | `uint32` | いいえ | DNS TTL（秒単位）。 |
| `comment` | `string` | いいえ | DNS レコードコメント。Cloudflare のみ対応。 |

## レコード解決

### 明示的な値の場合

`value` が設定されている場合、Composia はそれを直接使用します。IP アドレスの場合、レコードタイプが推論されます: IPv4 は `A`、IPv6 は `AAAA` になります。ホスト名の場合、レコードタイプは `CNAME` である必要があります（空の場合も `CNAME` に解決されます）。

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    value: 203.0.113.10
```

### ノードアドレスから解決

`value` が空の場合、Composia はコントローラー設定のターゲットノードの `public_ipv4` と `public_ipv6` を使用します:

```yaml
controller:
  nodes:
    - id: "main"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
```

`record_type` が空の場合、ノードが両方のアドレスを持つときに A レコードと AAAA レコードの両方が作成されます。`record_type` が `A` の場合、IPv4 アドレスのみが使用されます。`record_type` が `AAAA` の場合、IPv6 アドレスのみが使用されます。

複数のノードをターゲットとするサービスは `value` を明示的に設定する必要があります。複数のターゲットノードがある場合に `value` が空だとエラーになります。

## DNS 更新のトリガー

DNS レコードはデプロイタスクフロー中に作成または更新されます。Web UI または CLI から独立した DNS 更新をトリガーすることもできます:

```bash
composia service dns-update my-app
```

これは `dns_update` タスクを作成します。タスクログにはゾーン解決、レコード操作、最終結果が表示されます。

## Cloudflare オプション

プロバイダーが `cloudflare` の場合、`proxied` と `comment` はレコード作成後に適用されます。Composia は Cloudflare API を呼び出して各 DNS レコードを要求されたプロキシステータスとコメントでパッチします。

Cloudflare 以外のプロバイダーはこれらのオプションをサポートしていません。他のプロバイダーで `proxied` または `comment` を設定すると DNS 更新が失敗します。

## ゾーンマッチング

Composia はサービスホスト名を設定されたゾーンと照合します。ゾーンは最も長い一致から最も短い一致の順に試行されます。例えば `zones: ["example.com.", "sub.example.com."]` の場合、ホスト名 `app.sub.example.com` は最初に `sub.example.com.` にマッチします。

ホスト名に一致するゾーンがない場合、DNS 更新は失敗します。

## 古いレコードのクリーンアップ

DNS 同期はホスト名ごとにちょうど 3 つのレコードタイプ（A、AAAA、CNAME）を管理します。期待状態に存在しない設定済みレコードタイプは、新しいレコードが設定される前に削除されます。例えば、以前 `record_type: A` だったサービスが `record_type: CNAME` に変更された場合、古い A レコードが削除され新しい CNAME レコードが作成されます。

サービスのホスト名を変更しても、古いホスト名のレコードはクリーンアップされません。`app.example.com` を `api.example.com` にリネームした場合、`app.example.com` のレコードは手動で削除するまで DNS プロバイダーに残ります。
