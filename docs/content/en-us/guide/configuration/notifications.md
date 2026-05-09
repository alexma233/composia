# Notification Configuration

This page documents the `controller.notifications` configuration for built-in SMTP and Telegram notification channels.

## Controller-Side Configuration

Place the `notifications` block under the `controller` section in `config/config.yaml`.

## Example

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

## Fields

### SMTP Channel

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether to enable SMTP notifications. Defaults to `true` when the section is present |
| `host` | string | SMTP server hostname |
| `port` | int | SMTP server port (1-65535) |
| `encryption` | string | Encryption mode: `none`, `starttls`, or `ssl_tls`. Defaults to `starttls` |
| `username` | string | SMTP authentication username. Omit for unauthenticated relay |
| `password` | string | SMTP authentication password |
| `from` | string | Sender address |
| `to` | []string | Recipient addresses |
| `on` | []string | Event types to notify on |
| `task_sources` | []string | Filter by task source: `web`, `cli`, `others`, `schedule`, `system`. Omit to include all sources |

If `enabled` is `true` (or omitted), `host`, `port`, `from`, and at least one entry in `to` are required.

### Telegram Channel

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether to enable Telegram notifications. Defaults to `true` when the section is present |
| `bot_token` | string | Telegram Bot API token |
| `chat_id` | string | Target chat or channel ID |
| `on` | []string | Event types to notify on |
| `task_sources` | []string | Filter by task source. Omit to include all sources |

If `enabled` is `true` (or omitted), `bot_token` and `chat_id` are required.

## Supported Event Types

| Event | Trigger |
|-------|---------|
| `task_failed` | Any task terminates with a failed status |
| `task_cancelled` | Any task is cancelled |
| `task_completed` | Any task succeeds |
| `task_awaiting_confirmation` | A migrate task enters human-approval state |
| `backup_completed` | A backup data item succeeds |
| `backup_failed` | A backup data item fails |
| `image_update_available` | A new image version is detected |
| `image_update_applied` | An auto-apply image update task is queued |
| `node_offline` | A node exceeds the heartbeat timeout |
| `node_online` | A previously-offline node sends a heartbeat |
| `alertmanager_alert` | An Alertmanager-compatible webhook alert is received |

The `task_sources` filter only applies to task-derived events (`task_*`, `backup_*`, `image_update_*`). Node and Alertmanager events ignore the source filter.

## Alertmanager Webhook

You can also forward Alertmanager alerts through the same notification channels:

```yaml
controller:
  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      on: [alertmanager_alert]
```

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Whether to register the webhook endpoint. Defaults to `true` when present |
| `listen_path` | string | HTTP path to listen on. Defaults to `/api/v1/alerts` |

Once enabled, POST Alertmanager-compatible JSON to the controller's HTTP port at the configured path. Each alert is dispatched through notification channels that have `alertmanager_alert` in their `on` list.

## Related Documentation

- [Configuration Guide](../configuration) — All config sections overview
- [Operations](../operations) — Production operations and monitoring
- [Service Definition](../service-definition) — Service-side configuration
