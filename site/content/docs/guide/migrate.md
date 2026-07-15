---
title: "Migrate"
date: '2026-05-26T00:00:00+08:00'
weight: 45
---

Migrate a service from one node to another while preserving data integrity. The migration task orchestrates backup, stop, restore, start, and DNS update steps across source and target nodes.

## Configuration

Data items carried during migration must have both a `backup` and a `restore` action in `data_protect`. Declare them in `migrate`:

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

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Must reference a `data_protect.data[].name` with both backup and restore actions. |
| `enabled` | `bool` | No | Enable or disable migration for this item. |

## Execute migration

**Web UI:**
1. Open the service detail page.
2. Use the migration controls to select source and target nodes.
3. Click **Migrate**.

**CLI:**

```bash
composia service my-app migrate --source main --target edge-1 --wait --follow --timeout 30m
```

## Migration steps

1. **Export data** — run a backup task on the source node for each configured data item.
2. **Stop source instance** — run `docker compose down`, remove Caddy configuration.
3. **Reload Caddy on source** — remove the proxy entry from the source Caddy instance.
4. **Restore data on target** — run a restore task on the target node for each data item.
5. **Deploy on target** — run `docker compose up -d`, sync Caddy configuration.
6. **Reload Caddy on target** — apply the proxy entry on the target Caddy instance.
7. **Update DNS** — update DNS records to point to the target node.
8. **Write configuration** — update `nodes` in `composia-meta.yaml`, commit to Git.

## Considerations

- The service must be deployed on the source node and the target node must be online.
- Migration causes brief downtime. Perform during off-peak hours.
- Source instance is stopped before data transfer to ensure consistency.
- For databases, use export strategies (`database.pgdumpall` / `database.pgimport`).

## Rollback

State rollback is currently available in the Web UI only. Open the migration task details, choose the recovery actions that match the failed step, and start rollback there.

| Action | Description |
|--------|-------------|
| `deploy_source` | Redeploy the service on the original source node. |
| `stop_target` | Stop and clean up the service on the target node. |
| `rollback_dns` | Sync DNS records back to the source node. |

The CLI does not have a `task rollback` command yet. You can still inspect and follow the migration task with:

```bash
composia task wait <task-id> --follow --timeout 30m
```

## See also

- [Backups](/docs/guide/backups/) — Rustic setup and backup configuration.
- [Service Configuration](/docs/guide/service/) — `data_protect` and `migrate` field reference.
