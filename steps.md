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

**当前状态**: rustic backup/restore 已成功测试；`database.pgdumpall` 已有 agent 自动化覆盖但仍需真实环境验证；`database.pgimport` 已实现但缺自动化和真实环境验证；`ForgetNodeRustic` / `PruneNodeRustic` 已成功测试。定时备份与 rustic maintenance 已有调度层自动化覆盖，仍需真实环境回归。

**待完成:**

1. `database.pgdumpall` backup strategy 真实环境测试与失败场景测试
2. `database.pgimport` restore strategy 自动化测试、真实环境测试与失败场景测试
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

**当前状态**: controller migrate task 已按“源节点 stop -> backup -> 目标 restore -> 目标 deploy -> 目标 Caddy reload -> awaiting_confirmation”的顺序执行；确认通过会重新入队继续 DNS 更新与 `persist_repo`，确认失败会取消任务并停止后续步骤。Web 任务详情页已有基础确认/拒绝入口，服务详情页已有源节点和目标节点输入。

**待完成:**

1. `persist_repo` 冲突处理：当 `HEAD` 变化且触及该服务目录时的专门冲突语义（当前只有通用 base revision / clean worktree 保护）
2. 整个 migrate 流程近生产环境端到端测试
3. 迁移回滚 task/type 实现
4. Web UI 迁移确认体验增强（确认说明、拒绝原因、恢复后状态反馈）

---

## 3. Web UI 增强

**当前状态**: 备份列表/详情与 restore 入口已存在；任务列表支持 `awaiting_confirmation` 筛选；任务详情页支持 migrate 确认/拒绝；Docker container/image/network/volume 页面已按节点维度拆出。

1. **Service 管理页面重构**: 继续拆分文件编辑、实例管理、操作入口为更清晰的 multi-node 架构视图
2. **CodeMirror 编辑器增强**: 添加 `composia-meta.yaml` 语义检查
3. **Restore 交互打磨**: 备份详情页 restore 真实环境反馈、失败呈现和恢复后状态追踪
4. **`awaiting_confirmation` UI 打磨**: 任务详情页确认说明、拒绝原因输入、筛选呈现和操作流

---

## 4. DNS、Caddy 和容器操作收尾

**当前状态**: `dns_update` 在 `network.dns.value` 为空且服务不止一个目标节点时会拒绝执行，避免继续按单节点推导；节点级 Caddy reload/sync API 与 Web 入口已存在；容器列表、详情、日志和基础操作已按节点维度存在。

1. `caddy` 作为多节点 service 的实例扇出语义
2. `migrate` 完成后的源/目标节点 `caddy_reload` 串联（当前 migrate 只在目标 deploy 后 reload 目标节点）

---

## 5. CLI 命令面实现

当前已拆分为 `composia` 用户 CLI、`composia-controller` 和 `composia-agent` 三个入口。用户 CLI 已覆盖主要 controller RPC、repo/secret 写入和本地配置；异步动作支持 `--wait` / `--follow` / `--timeout`，并可用 `composia task wait` 单独等待任务。

```text
system status
service list/get/deploy/stop/restart/update/backup/dns-update/caddy-sync/migrate
instance list/get/deploy/stop/restart/update/backup
container list/get/logs/start/stop/restart/remove/exec
task list/get/logs/wait/run-again/approve/reject
backup list/get/restore
node list/get/tasks/reload-caddy/prune
repo head/files/get/edit/update/history/sync/validate
secret get/edit/update
config get/set/unset/path
```

后续 CLI 仍可完善：

1. `container exec` 目前只创建 exec session 并输出 websocket path，尚未提供本地 TTY attach 体验。
2. Docker network/volume/image 的 list/inspect/remove RPC 已存在，但 CLI 尚未公开成稳定命令面。
3. Rustic 节点维护 RPC 已存在，但 CLI 尚未公开 `rustic init/forget/prune`。
4. Shell completion 尚未实现。

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
