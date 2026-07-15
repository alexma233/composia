---
title: "迁移"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

将服务从一个节点迁移到另一个节点，同时保持数据完整性。迁移任务协调源节点和目标节点之间的备份、停止、恢复、启动和 DNS 更新步骤。

## 配置

迁移期间携带的数据项必须在 `data_protect` 中同时具有 `backup` 和 `restore` 操作。在 `migrate` 中声明它们：

```yaml
name: my-app
nodes:
  - main

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

migrate:
  data:
    - name: uploads
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必须引用同时具有备份和恢复操作的 `data_protect.data[].name`。 |
| `enabled` | `bool` | 否 | 启用或禁用此项的迁移。 |

## 执行迁移

**Web UI：**
1. 打开服务详情页面。
2. 使用迁移控件选择源节点和目标节点。
3. 点击**迁移**。

**CLI：**

```bash
composia service my-app migrate --source main --target edge-1 --wait --follow --timeout 30m
```

## 迁移步骤

1. **导出数据** — 在源节点上为每个已配置的数据项运行备份任务。
2. **停止源实例** — 运行 `docker compose down`，移除 Caddy 配置。
3. **在源节点重载 Caddy** — 从源 Caddy 实例中移除代理条目。
4. **在目标节点恢复数据** — 在目标节点上为每个数据项运行恢复任务。
5. **在目标节点部署** — 运行 `docker compose up -d`，同步 Caddy 配置。
6. **在目标节点重载 Caddy** — 在目标 Caddy 实例上应用代理条目。
7. **更新 DNS** — 更新 DNS 记录以指向目标节点。
8. **写入配置** — 更新 `composia-meta.yaml` 中的 `nodes`，提交到 Git。

## 注意事项

- 服务必须已部署在源节点上，且目标节点必须在线。
- 迁移会导致短暂停机。请在低峰时段执行。
- 源实例在数据传输前会停止以保证一致性。
- 对于数据库，请使用导出策略（`database.pgdumpall` / `database.pgimport`）。

## Rollback

State rollback is currently available in the Web UI only. Open the migration task details, choose the recovery actions that match the failed step, and start rollback there.

| Action | Description |
|--------|-------------|
| `deploy_source` | Redeploy the service on the original source node. |
| `stop_target` | Stop and clean up the service on the target node. |
| `rollback_dns` | Sync DNS records back to the source node. |

The CLI does not have a `task rollback` command yet. You can still inspect and follow the migration task with:

```bash
composia task wait --follow --timeout 30m <task-id>
```

## 另请参阅

- [备份](/docs/guide/backups/) — Rustic 设置与备份配置。
- [服务配置](/docs/guide/service/) — `data_protect` 和 `migrate` 字段参考。
