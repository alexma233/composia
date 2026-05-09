# 通知配置

本文档说明 `controller.notifications` 内置 SMTP 与 Telegram 通知通道的配置方式。

## Controller 侧配置

将 `notifications` 块放置在 `config/config.yaml` 的 `controller` 节下面。

## 示例

```yaml
controller:
  notifications:
    smtp:
      enabled: true
      host: smtp.example.com
      port: 465
      encryption: ssl_tls
      username: "bot@example.com"
      password: "secret"
      from: "bot@example.com"
      to:
        - "admin@example.com"
      on:
        - task_failed
        - backup_failed
      task_sources:
        - schedule

    telegram:
      enabled: true
      bot_token: "123456:ABC-..."
      chat_id: "-1001234567890"
      on:
        - task_failed
        - node_offline
        - alertmanager_alert
```

## 字段说明

### SMTP 通道

| 字段 | 类型 | 说明 |
|-------|------|-------------|
| `enabled` | bool | 是否启用 SMTP 通知。存在此节时默认为 `true` |
| `host` | string | SMTP 服务器主机名 |
| `port` | int | SMTP 服务器端口（1-65535） |
| `encryption` | string | 加密模式：`none`、`starttls` 或 `ssl_tls`。默认为 `starttls` |
| `username` | string | SMTP 认证用户名。省略表示无需认证 |
| `password` | string | SMTP 认证密码 |
| `from` | string | 发件人地址 |
| `to` | []string | 收件人地址列表 |
| `on` | []string | 需要通知的事件类型 |
| `task_sources` | []string | 按任务来源过滤：`web`、`cli`、`others`、`schedule`、`system`。省略表示所有来源 |

当 `enabled` 为 `true`（或省略）时，`host`、`port`、`from` 和 `to` 中至少一项为必填。

### Telegram 通道

| 字段 | 类型 | 说明 |
|-------|------|-------------|
| `enabled` | bool | 是否启用 Telegram 通知。存在此节时默认为 `true` |
| `bot_token` | string | Telegram Bot API token |
| `chat_id` | string | 目标聊天或频道 ID |
| `on` | []string | 需要通知的事件类型 |
| `task_sources` | []string | 按任务来源过滤。省略表示所有来源 |

当 `enabled` 为 `true`（或省略）时，`bot_token` 和 `chat_id` 为必填。

## 支持的事件类型

| 事件 | 触发条件 |
|-------|---------|
| `task_failed` | 任务以失败状态结束 |
| `task_cancelled` | 任务被取消 |
| `task_completed` | 任务成功完成 |
| `task_awaiting_confirmation` | 迁移任务进入等待人工确认状态 |
| `backup_completed` | 备份数据项成功 |
| `backup_failed` | 备份数据项失败 |
| `image_update_available` | 检测到新镜像版本 |
| `image_update_applied` | 自动应用的镜像更新任务已排队 |
| `node_offline` | 节点超过心跳超时 |
| `node_online` | 之前离线的节点发送心跳 |
| `alertmanager_alert` | 收到 Alertmanager 兼容的 webhook 告警 |

`task_sources` 过滤器仅对任务衍生事件（`task_*`、`backup_*`、`image_update_*`）生效。节点和 Alertmanager 事件不受来源过滤器限制。

## Alertmanager Webhook

你也可以通过相同的通知通道转发 Alertmanager 告警：

```yaml
controller:
  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      on: [alertmanager_alert]
```

| 字段 | 类型 | 说明 |
|-------|------|-------------|
| `enabled` | bool | 是否注册 webhook 端点。存在时默认为 `true` |
| `listen_path` | string | 监听 HTTP 路径。默认为 `/api/v1/alerts` |

启用后，向 Controller HTTP 端口配置的路径 POST Alertmanager 兼容的 JSON。每条告警通过 `on` 列表中包含 `alertmanager_alert` 的通知通道进行分发。

## 相关文档

- [配置指南](../configuration) —— 全部配置块总览
- [日常运维](../operations) —— 生产运维与监控
- [服务定义](../service-definition) —— 服务侧配置
