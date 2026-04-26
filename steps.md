# Composia — 待完成事项

> Multi-node 架构主干已完成。以下仅列出仍需推进的工作。

---

## 执行规则

1. 保持实现与当前文档化的 v1 语义一致，即使需要收紧或替换现有占位符行为。
2. 不再向旧的单节点 service 语义继续叠加功能。
3. 不要把容器操作继续塞进单节点 service API；应改为资源清晰的 instance/container API。
4. 将迁移、备份、DNS、secrets 和 repo 写入视为架构敏感工作，必须匹配文档化的 v1 语义，而不是快捷变体。

---

## 1. 备份与恢复：补齐真实环境验证与重构

**当前状态**: rustic backup/restore 已成功测试（除 `pgdump`）；`ForgetNodeRustic` / `PruneNodeRustic` 已成功测试。

**待完成:**

1. `database.pgdumpall` backup strategy 真实环境测试与失败场景测试
2. `database.pgimport` restore strategy 真实环境测试与重构
3. `files.copy`、`files.copy_after_stop` 真实环境测试与失败场景测试
4. restore 链路端到端回归测试（覆盖语义、数据库导入语义、失败后残留状态处理）
5. 每个 backup/restore strategy 明确一致性边界、错误处理、恢复语义和日志语义
6. 定时备份执行的真实环境回归

---

## 2. 迁移 (migrate)

**目标工作流:**

1. 源节点 service instance 停止
2. 源节点数据导出（停机后冷备份，使用 `migrate.data[]` 引用的 `backup` 定义）
3. 目标节点数据恢复（使用 `restore` 定义）
4. 目标节点 service instance 启动
5. 目标节点 Caddy reload（当 service 启用 `network.caddy`）
6. 进入 `awaiting_confirmation`
7. 人工验证通过后执行 DNS 更新
8. `persist_repo`：修改 `composia-meta.yaml.nodes` 并 commit/push
9. 人工验证失败时，当前 migrate task 直接结束，不继续执行 DNS 更新和 `persist_repo`
10. 回滚使用单独 task/type 编排，不复用当前 migrate task

**待完成:**

1. 调整 migrate 工作流顺序为"先停后备份"（当前为"先备份后停"）
2. `awaiting_confirmation` 挂起/恢复流程走真实路径（不允许快捷执行到完成）
3. `persist_repo` 冲突处理：当 `HEAD` 变化且触及该服务目录时的专门冲突语义
4. 整个 migrate 流程近生产环境端到端测试
5. 迁移回滚 task/type 实现
6. Web UI 迁移确认体验增强（确认流程、目标节点选择）

---

## 3. Web UI 增强

1. **Service 管理页面重构**: 拆分文件编辑、实例管理、操作入口为更清晰的 multi-node 架构视图
2. **CodeMirror 编辑器增强**: 添加 `composia-meta.yaml` 语义检查
3. **Restore 交互打磨**: 备份详情页 restore 入口交互与真实环境反馈
4. **`awaiting_confirmation` UI 打磨**: 任务详情页确认流程、筛选呈现和操作流

---

## 4. DNS、Caddy 和容器操作收尾

1. `caddy` 作为多节点 service 的实例扇出语义
2. `dns_update` 多节点 service 边缘情况：当 `network.dns.value` 为空时按单节点推导不适用多节点场景
3. `migrate` 完成后的源/目标节点 `caddy_reload` 串联

---

## 5. CLI 命令面实现

当前仅实现 `composia controller` 和 `composia agent` 两个入口。仍需实现面向用户的命令面：

```text
service list/get/deploy/stop/restart/update/backup/migrate
instance list/get/deploy/stop/restart/update/backup
container list/get/logs/start/stop/restart/exec
task list/get/run-again/logs
backup list/get
node list/get/tasks/reload-caddy/prune
repo head/files/get/update/history/sync
secret get/update
system status
dns update
```

---

## 6. 其他待办

1. `auto_deploy` 选项：repo 变更后自动触发 instance 扇出部署任务
2. `UpdateServiceTargetNodes` 在迁移场景下 `persist_repo` 的冲突语义完善

---

## 架构要点提醒

- 不要再扩展旧的单节点 service 语义。
- `Service` 不是部署实例；部署实例必须建模为 `ServiceInstance`。
- `Container` 是一等对象，不只是 node 的一个附属列表。
- 仅按文档化的工作流实现 `migrate`，不是快捷变体。
- 保持 controller 作为持久状态所有者，agent 作为执行端。
- 所有 repo 写入必须经过 repo lock 串行化。
- 所有 instance 和 container 动作都必须保留 `node_id` 这一维。
