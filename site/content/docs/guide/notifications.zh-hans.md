---
title: "通知"
date: '2026-05-26T00:00:00+08:00'
weight: 70
---

Composia 为任务结果、备份事件、镜像更新和节点状态变化发送通知。支持三种通知渠道：Alertmanager、SMTP 和 Telegram。

## 配置

所有渠道均在控制器配置的 `notifications` 下进行配置：

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

## 事件类型

以下通知事件类型可用：

| 事件 | 触发条件 |
|-------|---------|
| `task_failed` | 任何任务以 `failed` 状态结束。 |
| `task_cancelled` | 任务在完成前被取消。 |
| `task_completed` | 任务成功完成。 |
| `task_awaiting_confirmation` | 迁移任务到达确认步骤。 |
| `backup_completed` | 备份任务或定时备份成功完成。 |
| `backup_failed` | 备份任务或步骤失败。 |
| `image_update_available` | 镜像检查发现新版本。 |
| `image_update_applied` | 镜像更新已应用。 |
| `node_offline` | 节点停止发送心跳。 |
| `node_online` | 之前离线的节点恢复心跳。 |
| `alertmanager_alert` | 控制器作为 Alertmanager webhook 接收器时收到告警。 |

每个渠道可以通过 `on` 列表过滤应处理的事件类型。空的 `on` 列表表示接收所有事件类型。

## 任务来源过滤器

SMTP 和 Telegram 渠道支持按触发任务来源进行过滤：

| 来源 | 描述 |
|--------|-------------|
| `web` | 通过 Web UI 触发的操作。 |
| `cli` | 通过 CLI 触发的操作。 |
| `others` | 其他来源。 |
| `schedule` | 调度的任务（备份、维护）。 |
| `system` | 系统生成的任务。 |
| `auto_deploy` | 自动部署触发器生成的任务。 |

当 `task_sources` 为空时，所有来源类型都会发送通知。

## Alertmanager

控制器运行一个内嵌的 Alertmanager webhook 接收器。启用后，接收器在配置的路径上监听：

```yaml
alertmanager:
  enabled: true
  listen_path: "/api/v1/alerts"
```

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `enabled` | `bool` | 当该部分存在时默认启用。 |
| `listen_path` | `string` | 接收 Alertmanager webhook 的 HTTP 路径。默认为 `/api/v1/alerts`。必须以 `/` 开头且不含空白字符。 |

将您的 Alertmanager 实例指向控制器的地址并使用此 webhook URL。告警会根据事件过滤器转发到已配置的通知渠道。

## SMTP

SMTP 通过邮件发送通知：

| 键 | 类型 | 启用时必填 | 描述 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 当该部分存在时默认启用。 |
| `host` | `string` | 是 | SMTP 服务器主机名。 |
| `port` | `int` | 是 | SMTP 端口，1 到 65535。 |
| `encryption` | `string` | 否 | `none`、`starttls` 或 `ssl_tls`。默认为 `starttls`。 |
| `username` | `string` | 否 | SMTP 认证用户名。 |
| `password` | `string` | 否 | SMTP 密码。 |
| `password_file` | `string` | 否 | 从文件读取密码。 |
| `from` | `string` | 是 | 发件人地址。 |
| `to` | `[]string` | 是 | 收件人地址列表。 |
| `on` | `[]string` | 否 | 要通知的事件类型。 |
| `task_sources` | `[]string` | 否 | 任务来源过滤器。 |

## Telegram

Telegram 通过机器人向聊天发送通知：

| 键 | 类型 | 启用时必填 | 描述 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 当该部分存在时默认启用。 |
| `bot_token` | `string` | 是 | 从 BotFather 获取的 Telegram 机器人令牌。 |
| `bot_token_file` | `string` | 否 | 从文件读取机器人令牌。 |
| `chat_id` | `string` | 是 | 目标聊天 ID。 |
| `on` | `[]string` | 否 | 要通知的事件类型。 |
| `task_sources` | `[]string` | 否 | 任务来源过滤器。 |
