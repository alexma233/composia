---
title: "通知"
date: '2026-05-26T00:00:00+08:00'
weight: 70
---

Composia はタスク結果、バックアップイベント、イメージ更新、ノード状態変化の通知を送信します。Alertmanager、SMTP、Telegram の 3 つの通知チャンネルがサポートされています。

## 設定

すべてのチャンネルはコントローラー設定の `notifications` で設定します:

```yaml
controller:
  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: true
      host: "smtp.example.com"
      port: 587
      encryption: starttls
      username: "alerts@example.com"
      password: "REPLACE"
      from: "alerts@example.com"
      to:
        - "admin@example.com"
      on:
        - task_failed
        - backup_failed
      task_sources:
        - web
        - cli
    telegram:
      enabled: true
      bot_token: "REPLACE"
      chat_id: "REPLACE"
      on:
        - task_completed
```

## イベントタイプ

以下の通知イベントタイプが利用可能です:

| イベント | トリガー |
|-------|---------|
| `task_failed` | タスクが `failed` ステータスで終了した場合。 |
| `task_cancelled` | タスクが完了前にキャンセルされた場合。 |
| `task_completed` | タスクが正常に完了した場合。 |
| `task_awaiting_confirmation` | 移行タスクが確認ステップに到達した場合。 |
| `backup_completed` | バックアップタスクまたはスケジュールバックアップが正常に完了した場合。 |
| `backup_failed` | バックアップタスクまたはステップが失敗した場合。 |
| `image_update_available` | イメージチェックが新しいバージョンを検出した場合。 |
| `image_update_applied` | イメージ更新が適用された場合。 |
| `node_offline` | ノードがハートビートの送信を停止した場合。 |
| `node_online` | 以前オフラインだったノードがハートビートを再開した場合。 |
| `alertmanager_alert` | コントローラーが Alertmanager webhook レシーバーとして設定されている場合に Alertmanager アラートを受信した場合。 |

各チャンネルは `on` リストを使用して処理するイベントタイプをフィルタリングできます。空の `on` リストはすべてのイベントタイプを配信します。

## タスクソースフィルター

SMTP と Telegram チャンネルはタスクをトリガーしたソースによるフィルタリングをサポートします:

| ソース | 説明 |
|--------|-------------|
| `web` | Web UI を通じてトリガーされたアクション。 |
| `cli` | CLI を通じてトリガーされたアクション。 |
| `others` | その他のソース。 |
| `schedule` | スケジュールされたタスク（バックアップ、メンテナンス）。 |
| `system` | システム生成タスク。 |
| `auto_deploy` | 自動デプロイトリガーによって生成されたタスク。 |

`task_sources` が空の場合、すべてのソースタイプに対して通知が送信されます。

## Alertmanager

コントローラーは組み込みの Alertmanager webhook レシーバーを実行します。有効にすると、レシーバーは設定されたパスで待機します:

```yaml
alertmanager:
  enabled: true
  listen_path: "/api/v1/alerts"
```

| キー | 型 | 説明 |
|-----|------|-------------|
| `enabled` | `bool` | セクションが存在する場合、デフォルトで有効。 |
| `listen_path` | `string` | Alertmanager webhook を受信する HTTP パス。デフォルトは `/api/v1/alerts`。`/` で始まり、空白を含まない必要があります。 |

この webhook URL で Alertmanager インスタンスをコントローラーのアドレスに向けます。アラートはイベントフィルターに従って設定された通知チャンネルに転送されます。

## SMTP

SMTP はメールで通知を配信します:

| キー | 型 | 有効時に必須 | 説明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | いいえ | セクションが存在する場合、デフォルトで有効。 |
| `host` | `string` | はい | SMTP サーバーホスト名。 |
| `port` | `int` | はい | SMTP ポート。1 から 65535 の間である必要があります。 |
| `encryption` | `string` | いいえ | `none`、`starttls`、`ssl_tls`。デフォルトは `starttls`。 |
| `username` | `string` | いいえ | SMTP 認証ユーザー名。 |
| `password` | `string` | いいえ | SMTP パスワード。 |
| `password_file` | `string` | いいえ | ファイルからパスワードを読み取ります。 |
| `from` | `string` | はい | 送信者アドレス。 |
| `to` | `[]string` | はい | 受信者アドレス。 |
| `on` | `[]string` | いいえ | 通知するイベントタイプ。 |
| `task_sources` | `[]string` | いいえ | タスクソースフィルター。 |

## Telegram

Telegram はボットを通じてチャットに通知を送信します:

| キー | 型 | 有効時に必須 | 説明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | いいえ | セクションが存在する場合、デフォルトで有効。 |
| `bot_token` | `string` | はい | BotFather からの Telegram ボットトークン。 |
| `bot_token_file` | `string` | いいえ | ファイルからボットトークンを読み取ります。 |
| `chat_id` | `string` | はい | ターゲットチャット ID。 |
| `on` | `[]string` | いいえ | 通知するイベントタイプ。 |
| `task_sources` | `[]string` | いいえ | タスクソースフィルター。 |
