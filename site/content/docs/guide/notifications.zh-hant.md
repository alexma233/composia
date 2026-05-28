---
title: "通知"
date: '2026-05-26T00:00:00+08:00'
weight: 70
---

Composia 為任務結果、備份事件、映像檔更新和節點狀態變更發送通知。支援三種通知管道：Alertmanager、SMTP 與 Telegram。

## 設定

所有管道都在控制器設定的 `notifications` 下設定：

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

## 事件型別

提供以下通知事件型別：

| 事件 | 觸發條件 |
|-------|---------|
| `task_failed` | 任何任務以 `failed` 狀態結束。 |
| `task_cancelled` | 任務在完成前被取消。 |
| `task_completed` | 任務成功完成。 |
| `task_awaiting_confirmation` | 遷移任務到達確認步驟。 |
| `backup_completed` | 備份任務或排程備份成功完成。 |
| `backup_failed` | 備份任務或步驟失敗。 |
| `image_update_available` | 映像檔檢查發現新版本。 |
| `image_update_applied` | 映像檔更新已套用。 |
| `node_offline` | 節點停止發送心跳。 |
| `node_online` | 先前離線的節點恢復心跳。 |
| `alertmanager_alert` | 當控制器設定為 Alertmanager webhook 接收器時收到 Alertmanager 警報。 |

每個管道可以使用 `on` 清單過濾應處理的事件型別。空的 `on` 清單會發送所有事件型別。

## 任務來源過濾器

SMTP 和 Telegram 管道支援按觸發任務的來源進行過濾：

| 來源 | 說明 |
|--------|-------------|
| `web` | 透過 Web UI 觸發的操作。 |
| `cli` | 透過 CLI 觸發的操作。 |
| `others` | 其他來源。 |
| `schedule` | 排程任務（備份、維護）。 |
| `system` | 系統產生的任務。 |
| `auto_deploy` | 自動部署觸發器產生的任務。 |

當 `task_sources` 為空白時，所有來源型別都會發送通知。

## Alertmanager

控制器執行一個內嵌的 Alertmanager webhook 接收器。啟用後，接收器在設定的路徑上監聽：

```yaml
alertmanager:
  enabled: true
  listen_path: "/api/v1/alerts"
```

| 鍵 | 型別 | 說明 |
|-----|------|-------------|
| `enabled` | `bool` | 當區段存在時預設啟用。 |
| `listen_path` | `string` | 接收 Alertmanager webhook 的 HTTP 路徑。預設為 `/api/v1/alerts`。必須以 `/` 開頭且不含空白字元。 |

將您的 Alertmanager 執行實例指向控制器的位址及此 webhook URL。警報會根據其事件過濾器轉發到已設定的通知管道。

## SMTP

SMTP 透過電子郵件發送通知：

| 鍵 | 型別 | 啟用時必要 | 說明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 當區段存在時預設啟用。 |
| `host` | `string` | 是 | SMTP 伺服器主機名稱。 |
| `port` | `int` | 是 | SMTP 連接埠，必須在 1 到 65535 之間。 |
| `encryption` | `string` | 否 | `none`、`starttls` 或 `ssl_tls`。預設為 `starttls`。 |
| `username` | `string` | 否 | SMTP 驗證使用者名稱。 |
| `password` | `string` | 否 | SMTP 密碼。 |
| `password_file` | `string` | 否 | 從檔案讀取密碼。 |
| `from` | `string` | 是 | 寄件者地址。 |
| `to` | `[]string` | 是 | 收件者地址。 |
| `on` | `[]string` | 否 | 要通知的事件型別。 |
| `task_sources` | `[]string` | 否 | 任務來源過濾器。 |

## Telegram

Telegram 透過機器人向聊天室發送通知：

| 鍵 | 型別 | 啟用時必要 | 說明 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 當區段存在時預設啟用。 |
| `bot_token` | `string` | 是 | 來自 BotFather 的 Telegram 機器人權杖。 |
| `bot_token_file` | `string` | 否 | 從檔案讀取機器人權杖。 |
| `chat_id` | `string` | 是 | 目標聊天室 ID。 |
| `on` | `[]string` | 否 | 要通知的事件型別。 |
| `task_sources` | `[]string` | 否 | 任務來源過濾器。 |
