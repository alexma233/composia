---
title: "备份"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

Composia 通过 Rustic 实现自动备份。备份和恢复任务在 agent 上运行，而控制器负责生成运行时配置。

## 架构

备份需要一个 Rustic 基础设施服务。仓库中必须声明一个带有 `infra.rustic` 的服务：

```yaml {filename="rustic/composia-meta.yaml"}
name: rustic
nodes:
  - main
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
```

Rustic compose 服务是一个运行 `rustic` 二进制文件的普通 Docker 容器。它应该有一个用于数据保护目录的卷。

## 控制器配置

```yaml
controller:
  backup:
    default_schedule: "0 2 * * *"
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "0 1 * * Sun"
      prune_schedule: "0 3 * * Sun"
```

| 键 | 描述 |
|-----|-------------|
| `backup.default_schedule` | 服务备份的默认 cron 计划。 |
| `rustic.main_nodes` | 运行 Rustic 操作的节点 ID 列表。每个都必须引用已配置的节点。 |
| `rustic.maintenance.forget_schedule` | `rustic forget` 的 cron 计划。 |
| `rustic.maintenance.prune_schedule` | `rustic prune` 的 cron 计划。 |

## 服务数据保护

在 `composia-meta.yaml` 的 `data_protect` 下定义要备份的内容：

```yaml
data_protect:
  data:
    - name: db
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres
    - name: uploads
      backup:
        strategy: files.copy_after_stop
        include:
          - ./uploads
      restore:
        strategy: files.copy
        include:
          - ./uploads
```

### 数据策略

| 策略 | 用途 |
|----------|---------|
| `files.copy` | 复制文件和目录。用于可实时读取的数据。 |
| `files.copy_after_stop` | 停止 compose 项目，复制文件，然后重新启动。用于需要静止状态的数据。 |
| `database.pgdumpall` | 在 compose 服务内运行 `pg_dumpall`。需要设置 `service`。 |
| `database.pgimport` | 通过 `psql` 恢复 PostgreSQL 转储。需要设置 `service`。 |

### 数据操作字段

| 键 | 类型 | 适用于 | 描述 |
|-----|------|-------------|-------------|
| `strategy` | `string` | 全部 | 备份或恢复策略。 |
| `service` | `string` | `database.*` | Compose 服务名称。 |
| `include` | `[]string` | `files.*` | 要包含的路径，相对于服务目录。不能超出服务根目录。 |

### 包含路径类型

路径可以引用：

- **服务路径**: 服务目录内的文件或目录，直接复制。
- **命名卷**: Docker 卷名。通过启动一个挂载该卷的临时容器进行备份。

## 备份计划

为受保护的数据项启用定时备份：

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
    - name: uploads
      enabled: true
      schedule: "0 3 * * Sun"
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必须引用具有备份操作的 `data_protect.data[].name`。 |
| `provider` | `string` | 否 | 备份提供者名称。 |
| `enabled` | `bool` | 否 | 启用或禁用此备份。 |
| `schedule` | `string` | 否 | Cron 表达式。`"none"` 表示禁用调度但保留该条目。 |

当设置了 `schedule` 时，控制器会调度重复的 `backup` 任务。如果服务条目未指定自己的计划，则使用控制器的 `backup.default_schedule` 作为回退。

## 备份执行流程

备份任务在 agent 上执行以下步骤：

1. **渲染**: 从控制器下载服务包和 Rustic 包。读取由控制器生成的 `.composia-backup.json`。
2. **备份**: 对运行时配置中的每个数据项：
   - 根据备份策略（`files.copy`、`files.copy_after_stop`、`database.pgdumpall`）准备数据。
   - 运行 `docker compose run rustic backup`，并带上标识服务和数据项的标签。
   - 将结果（快照 ID）上报给控制器。
3. 所有项备份完成，任务结束。

备份产物由 Rustic 快照 ID 标识。标签包含 `composia-service:<name>` 和 `composia-data:<name>`，用于后续的恢复和清理操作。

## 恢复

通过 Web UI 的备份页面或 CLI 触发恢复：

```bash
composia backup restore <backup-id>
```

恢复过程：

1. **渲染**: 下载服务包和 Rustic 包。读取 `.composia-restore.json`。
2. **恢复**: 对每个项：
   - 运行 `docker compose run rustic restore <snapshot_id> <target_dir>`。
   - 根据恢复策略应用恢复的数据：
     - `files.copy`: 替换服务目录中的文件。
     - `files.copy_after_stop`: 停止 compose，替换文件，重启 compose。
     - `database.pgimport`: 使用恢复的 SQL 转储运行 `docker compose exec <service> psql`。

## Rustic 维护

维护任务使用 Rustic 基础设施服务：

- **`rustic_init`**: 运行 `docker compose run rustic init` 初始化 Rustic 仓库。每个 Rustic 设置只需使用一次。
- **`rustic_forget`**: 运行 `docker compose run rustic forget` 并带标签过滤器。作用域可以是某个服务、数据项或整个仓库。
- **`rustic_prune`**: 运行 `docker compose run rustic prune` 删除未引用的数据。

通过 Web UI 或 CLI 触发维护：

```bash
composia node init-rustic main
composia node forget-rustic main
composia node prune-rustic main
```

## 另请参阅

- [服务配置](/docs/guide/service/) — 数据保护和备份调度。
- [迁移](/docs/guide/migrate/) — 在节点之间移动服务并通过备份保留数据。
