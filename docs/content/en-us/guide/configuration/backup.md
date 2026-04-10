# Backup Configuration

This page documents the `controller.backup` and `controller.rustic` configuration.

## Example

```yaml
controller:
  backup:
    default_schedule: "0 2 * * *"
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "15 3 * * *"
      prune_schedule: "45 3 * * *"
```

You also need to deploy the rustic infrastructure service. See [Backup & Migration](../backup-migrate).

## Rules

- Each entry in `rustic.main_nodes` must reference an existing `controller.nodes[].id`
- `controller.backup.default_schedule` provides the default cron expression for service backup items
- `controller.rustic.maintenance.forget_schedule` and `controller.rustic.maintenance.prune_schedule` are only for rustic repository-wide maintenance tasks and cannot be overridden in service meta

## Service-Side Override

Service-side backup items may override the default schedule in `composia-meta.yaml`:

```yaml
backup:
  data:
    - name: uploads
      provider: rustic
      schedule: "0 */6 * * *"
    - name: cache
      provider: rustic
      schedule: none
```

Rules:

- When `backup.data[].schedule` is set, it overrides the controller default
- `schedule: none` disables automatic backup for that data item
- `forget` and `prune` always use controller-side configuration only
