# 备份配置

本文档说明 `controller.backup` 与 `controller.rustic` 配置。

## 配置示例

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

同时需要部署 rustic 基础设施服务，参考 [备份与迁移](../backup-migrate)。

## 规则

- `rustic.main_nodes` 中的每个节点 ID 都必须引用已存在的 `controller.nodes[].id`
- `controller.backup.default_schedule` 是所有服务备份项的默认定时表达式
- `controller.rustic.maintenance.forget_schedule` 和 `controller.rustic.maintenance.prune_schedule` 仅用于 rustic 仓库级维护任务，不能在 service meta 中覆盖

## 服务侧覆盖

服务侧可在 `composia-meta.yaml` 中覆盖单个备份项的定时：

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

规则如下：

- `backup.data[].schedule` 非空时，覆盖 controller 默认值
- `schedule: none` 表示该数据项永不自动备份
- `forget` 与 `prune` 始终只看 controller 配置
