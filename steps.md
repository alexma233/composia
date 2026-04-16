# Composia Implementation Steps

## 当前重构结论

从本次重构开始，仓库正式采用 multi-node 架构。

- `Service` 表示逻辑服务定义，不再表示某个单节点上的运行实体。
- `ServiceInstance` 表示一个服务在某个节点上的实际部署实例，主键为 `(service_name, node_id)`。
- `Container` 表示某个节点上的 Docker 容器，并通过 compose labels 关联到 `ServiceInstance`。
- `Node` 保持为 agent 和 Docker 宿主资源的管理对象。

这意味着旧的"一个 service 对应一个 node"假设全部废弃。凡是公开 API、数据库结构、repo schema、任务模型、Web UI 文案仍然携带该假设的地方，都必须被替换。

## 当前代码库状态

代码库已超越初始 scaffold 阶段，multi-node 主干已完成；当前剩余工作主要集中在 migration、restore、rustic forget/prune 的真实环境验证，以及部分 UI/编辑器增强。

当前风险说明：

- `restore`
- `migrate`
- `ForgetNodeRustic` / `PruneNodeRustic`

近期补充验证：

- 已在接近生产环境上持续使用，暂未发现巨大问题
- `rustic init` 已跑通
- Caddy 相关功能已确认正常

当前未完成的真实环境验证重点，已收敛到 `restore`、`migrate`、`ForgetNodeRustic`、`PruneNodeRustic`。

### 已实现或大部分实现

**核心基础设施：**
- `composia controller` 和 `composia agent` 子命令已存在。
- `config.yaml` 加载和验证已实现（controller 和 agent 模型）。
- Controller 启动初始化本地目录和打开 SQLite。
- Controller-agent ConnectRPC 基础链路已存在：heartbeat、long-poll task pull、bundle download、task state、step state、log upload、backup reporting、Docker stats reporting。
- Agent heartbeat 工作正常，节点状态已持久化。
- 服务 repo 扫描和 `composia-meta.yaml` 验证已存在。
- Docker 浏览 API（containers、networks、images、volumes）已存在，但仍然挂在 node 维度上。

**Controller Public API（已实现 multi-node 语义）：**
- `ServiceQueryService` - ListServices, GetService(含instances), GetServiceTasks, GetServiceBackups
- `ServiceCommandService` - UpdateServiceTargetNodes, RunServiceAction, MigrateService
- `ServiceInstanceService` - ListServiceInstances, GetServiceInstance, RunServiceInstanceAction
- `ContainerService` - RunContainerAction, GetContainerLogs, OpenContainerExec
- `TaskService`
- `BackupRecordService`
- `NodeQueryService` - ListNodes, GetNode, GetNodeTasks, GetNodeDockerStats
- `NodeMaintenanceService` - SyncNodeCaddyFiles, ReloadNodeCaddy, PruneNodeDocker, ForgetNodeRustic, PruneNodeRustic
- `DockerQueryService` - ListNodeContainers, InspectNodeContainer, ListNodeNetworks, InspectNodeNetwork, ListNodeVolumes, InspectNodeVolume, ListNodeImages, InspectNodeImage
- `RepoQueryService`
- `RepoCommandService`
- `SecretService`
- `SystemService`

**Agent 任务执行（已完成）：**
- `deploy`：完整实现（bundle download、render、compose-up、caddy-sync steps）
- `update`：完整实现（pull + compose-up + caddy-sync steps）
- `stop`：完整实现（download bundle、compose-down、remove generated caddy file）
- `restart`：完整实现（compose-down + compose-up）
- `backup`：完整实现（rustic、files.copy、files.tar_after_stop、database.pgdumpall）
- `prune`：完整实现（targets: all、containers、networks、images、volumes、builder）
- `dns_update`：Controller 端实现（Cloudflare 集成）
- `caddy_sync`：Agent 端实现（同步单 service Caddy 片段，或重建整个 node 的 generated_dir）
- `caddy_reload`：Agent 端实现（docker compose exec caddy reload）
- `deploy`/`update` 成功后自动为对应节点串联 `caddy_reload`（当 service 启用 `network.caddy`）
- `stop` 成功后自动删除对应 generated Caddy 片段，并为对应节点串联 `caddy_reload`

### 已完成的架构重构

以下单节点假设已全部修复，multi-node 架构现已完整：

1. `composia-meta.yaml` 使用 `nodes[]` 数组表示部署目标
2. `repo.Service` 使用 `TargetNodes []string` 表示多节点目标
3. `services` 表仅存储逻辑服务定义；运行时状态迁移到 `service_instances` 表
4. Agent 通过 `ReportServiceInstanceStatus` 上报 `service_name + node_id`
5. Service 动作通过 `RunServiceAction` + `node_ids[]` 实现 fan-out
6. `GetService` 返回完整的 `nodes[]` 和 `instances[]`
7. Web UI 已重切为 `service / service instance / container / node` 四层模型

### 一等对象实现状态

- `Service` - 已实现
- `ServiceInstance` - 已实现
- `Container` - 已实现
- `Node` - 已实现

---

## 执行规则

1. 保持实现与当前文档化的 v1 语义一致，即使需要收紧或替换现有占位符行为。
2. 优先完成 multi-node 主干，不再向旧的单节点 service 语义继续叠加功能。
3. 不要把容器操作继续塞进单节点 service API；应改为资源清晰的 instance/container API。
4. 将迁移、备份、DNS、secrets 和 repo 写入视为架构敏感工作，必须匹配文档化的 v1 语义，而不是快捷变体。

---

## Phase 1: 切换到 Multi-Node 语义

**状态：已完成**

目标：在扩展 Caddy 和容器操作之前，把当前后端从"单 service 对应单 node"切换到"service 定义 + service instance"契约。

已完成：
1. `composia-meta.yaml` 使用 `nodes[]` 数组表示部署目标。
2. `repo.Service` 持有 `TargetNodes []string`。
3. repo 验证、扫描、查找逻辑全部切换为 multi-node。
4. `services` 表仅表示逻辑服务；`service_instances` 表表示实例（主键 `service_name, node_id`）。
5. Agent 通过 `ReportServiceInstanceStatus` 上报 `service_name + node_id`。
6. `ServiceInstanceService` API 实现（ListServiceInstances, GetServiceInstance, RunServiceInstanceAction）。

---

## Phase 2: 完成 Multi-Target 任务基础

**状态：已完成**

目标：使任务系统可靠且严格符合 multi-node 的 v1 任务模型。

已完成：
1. service 动作通过 `RunServiceAction` + `node_ids[]` 进行 fan-out。
2. service 级冲突检查改为 service 和 instance 分层检查。
3. 任务详情保留 `service_name` 和 `node_id`。
4. `RunTaskAgain` 语义与 instance 目标保持一致。
5. `awaiting_confirmation` 保留用于未来真实的迁移工作流。

---

## Phase 3: 稳定第一个真实 ServiceInstance 操作

**状态：已完成**

目标：在添加更广泛工作流之前，完成已开始的 day-1 instance 操作。

已完成：
1. `deploy`、`update`、`stop`、`restart` 对单个 instance 的行为已稳定。
2. service 级动作作为批量入口，内部展开为 instance 动作。
3. Agent 报告的是 instance 运行时状态，而不是全局 service 状态。
4. 任务日志继续流式回传。

---

## Phase 4: 添加安全的 desired-state Repo 写入

**状态：已完成**

目标：让 controller 完全按照文档管理 Git-backed desired state 变更，并与 multi-node repo 语义对齐。

已完成：
1. Repo 写接口统一使用共享的写事务模板：串行 `repoMu`、写前 remote sync、`base_revision` 校验、dirty worktree 拒绝、active service task 冲突检查。
2. Repo 文件写入、目录创建、路径移动、路径删除、secret 写入都复用同一套 repo 写前检查和本地 commit 语义。
3. Repo 写事务内部已整理为清晰边界：写前检查、具体 mutation、Git 收尾（push/sync state）、declared services 刷新分别收口。
4. Push 失败时保留本地 commit，并继续通过 repo sync state 向 API/UI 报告 `push_failed`。
5. `ServiceCommandService.UpdateServiceTargetNodes` 已实现，controller 可以通过受控 API 定向改写 `composia-meta.yaml.nodes`，并复用现有 repo 写事务。
6. Repo lock 处理、验证、服务冲突检查和本地 commit 创建工作正常。
7. 可选远程同步行为、push 报告和 repo sync 状态工作正常。

待完成：
1. `UpdateServiceTargetNodes` 已可独立使用；迁移场景下的 `persist_repo` 仍需继续完善冲突语义（Phase 9）。
2. `auto_deploy` 选项（auto-deploy after repo changes）后续接到 instance 扇出任务创建。

---

## Phase 5: 添加 Secret 处理

**状态：已完成，需适配 multi-node bundle 语义**

目标：实现选定的 age-based secrets 模型，不在 `controller.repo_dir` 留下明文。

保留要求：
1. Controller 端解密和重新加密与配置的 age 设置一致。
2. Secret 写入与普通 repo 写入的 repo lock 和冲突语义一致。
3. 解密后的运行时文件只包含在 agent bundle 中，不持久化在 controller Git 工作树。
4. Git remote-sync 语义扩展到 secret 写入。

---

## Phase 6: 替换占位符备份行为

**状态：核心功能已完成；近生产环境验证已开始，但 restore 仍需系统性测试与重构**

目标：将当前备份 scaffold 转变为第一个真实数据保护工作流，并明确其 instance 语义。

已完成：
- 备份执行使用 rustic provider、files.copy、files.tar_after_stop、database.pgdumpall
- 备份记录持久化到 SQLite
- 备份查询 API 稳定
- service 级 backup 通过 `RunServiceAction` + `node_ids[]` 实现 instance 扇出

待完成：
1. 对 restore 链路做真实环境下的实际测试，并补齐 backup/restore 组合回归
2. 对每一个 backup strategy 单独补齐真实测试与失败场景测试：`files.copy`、`files.tar_after_stop`、`database.pgdumpall`
3. 对每一个 backup strategy 进行重构，明确一致性边界、错误处理、恢复语义和日志语义
4. restore 任务执行的真实回归测试与端到端校验
5. 迁移复用备份数据的工作流需要在真实环境下验证
6. `database.pgimport` restore 策略需要真实测试与重构
7. `files.untar` restore 策略需要真实测试与重构
8. 定时备份执行的真实环境回归

---

## Phase 7: 添加 Multi-Node Read-Write Web UI

**状态：核心功能已完成，增强功能待实现**

目标：保留现有真实 controller UI，但重切成 `service / service instance / container / node` 四层模型。

已完成：
- Dashboard、services、nodes、tasks、backups 页面连接到真实 API
- 任务日志 tailing
- Task list 页面支持 `status/serviceName/nodeId/type` 四项多选筛选，并同时支持 include/exclude 语义；后端 `ListTasks` API 与 SQLite 查询已同步支持多值 `IN/NOT IN`
- Web UI 任务视图默认排除低信号 Docker 查询任务：`docker_list`、`docker_inspect`、`docker_logs`
- Dashboard、node detail、service detail 的 recent tasks 区块已补充跳转到 task list 的入口，并自动带上对应筛选条件
- Repo 编辑和 sync 状态显示
- Secret 编辑
- Docker 浏览（containers、networks、images、volumes）
- Service 详情页显示 instances 列表和聚合状态
- ServiceInstance 详情页（通过 GetServiceInstance API）
- Container 详情页增加日志、start/stop/restart、terminal exec（基础实现）
- 任务 rerun UI 控件

待完成：
1. `MigrateService` UI 基础入口已实现；仍需继续增强确认流程和目标节点选择体验（Phase 9）。
2. 备份详情页和 restore 入口已实现；仍需继续打磨 restore 交互与真实环境反馈。
3. **Service 管理页面重构**：当前页面混合了文件编辑、实例管理、操作入口等多重职责，需要拆分为更清晰的 multi-node 架构视图。
4. **CodeMirror 编辑器增强**：基础编辑器、`.env` 检查、`docker compose` YAML 检查和分屏编辑能力已接入；仍缺少 `composia-meta.yaml` 语义检查等进一步增强。

---

## Phase 8: 添加 DNS、Caddy 和容器操作

**状态：核心功能已完成；Caddy 相关链路已在接近生产环境验证，`rustic init` 已跑通，`ForgetNodeRustic` / `PruneNodeRustic` 仍待实际测试**

目标：在 repo 写入和基础服务流稳定后，实现文档化的 day-2 操作动作，并将 Caddy 放在 multi-node service model 上。

已完成：
1. `dns_update` 任务行为 - controller 端 Cloudflare 集成
2. `prune` 任务行为 - all/containers/images/networks/volumes/builder targets
3. `caddy_sync` 任务行为 - 支持单 service 同步和 node 全量重建 generated Caddy files
4. `SyncNodeCaddyFiles` API
5. `caddy_reload` 任务行为
6. `ReloadNodeCaddy` API
7. `deploy`/`update` 成功后自动为对应节点同步 Caddy 文件并串联 `caddy_reload`
8. `stop` 成功后自动删除对应 generated Caddy 片段并串联 `caddy_reload`
9. `PruneNodeDocker` API
10. Container logs API - `GET /nodes/{id}/docker/containers/{cid}/logs`
11. Container start/stop/restart API - `POST /nodes/{id}/docker/containers/{cid}/actions/{action}`
12. Container exec session API - `POST /nodes/{id}/docker/containers/{cid}/exec`

待完成：
1. `caddy` 作为多节点 service 的实例扇出语义
2. `migrate` 完成后在源节点和目标节点都触发 `caddy_reload`（等待 Phase 9）
3. **Docker exec (terminal) 增强**：Web 端 terminal 与基础 exec 流程已接通，但目前还需要完整的测试和 review
4. `dns_update` 在 `network.dns.value` 为空时仍按单节点自动推导目标；多节点 service 若要统一 DNS，仍需显式提供 `network.dns.value`
5. `ForgetNodeRustic`、`PruneNodeRustic` 需要真实环境下的实际测试；当前不能视为已验证

---

## Phase 9: 最后添加迁移

**状态：核心能力已完成；确认挂起/恢复已接通，但 restore/migrate 仍待近生产环境端到端验证，冲突处理、回滚语义待完善**

目标：仅在整个平台语义稳定后，实现最复杂的 v1 工作流。

已完成：
1. `MigrateService(source_node_id, target_node_id)` RPC
2. `migrate` controller 侧编排任务骨架已实现，当前代码路径为：
   - 源节点数据导出（使用 `migrate.data[]` 引用的 `backup` 定义）
   - 源节点 service instance 停止
   - 源节点 Caddy reload（当 service 启用 `network.caddy`）
   - 目标节点数据恢复（使用 `restore` 定义）
   - 目标节点 service instance 启动
   - 目标节点 Caddy reload（当 service 启用 `network.caddy`）
   - DNS 更新
   - `persist_repo` 阶段：修改 `composia-meta.yaml.nodes` 并 commit/push
3. 通用 restore 基础能力已实现，并已被 migrate 复用
4. restore 当前支持：
   - `files.copy`
   - `files.untar`
   - `database.pgimport`
5. Web UI 已提供基础 `MigrateService` 入口

目标语义（待实现）：
1. `migrate` 的目标工作流改为：
   - 源节点 service instance 停止
   - 源节点数据导出（停机后冷备份，使用 `migrate.data[]` 引用的 `backup` 定义）
   - 目标节点数据恢复（使用 `restore` 定义）
   - 目标节点 service instance 启动
   - 目标节点 Caddy reload（当 service 启用 `network.caddy`）
   - 进入 `awaiting_confirmation`
   - 人工验证通过后再执行 DNS 更新
   - `persist_repo` 阶段：修改 `composia-meta.yaml.nodes` 并 commit/push
2. `awaiting_confirmation` 必须进入真实挂起/恢复工作流，不允许当前这种直接执行到完成的快捷路径
3. 人工验证失败时，当前 migrate task 直接结束，不继续执行 DNS 更新和 `persist_repo`
4. 回滚使用单独 task/type 编排，不复用当前 migrate task；该部分后续实现

待完善：
1. `ResolveTaskConfirmation(task_id, decision, comment)` 已实现，用于从 `awaiting_confirmation` 推进已有 task
2. `decision=approve` 时恢复同一个 migrate task 并继续执行 `dns_update -> persist_repo -> finalize` 已实现
3. `decision=reject` 时结束当前 migrate task，并记录 `manual verification rejected` 已实现
4. 迁移冲突处理：当 `persist_repo` 时 `HEAD` 已变化但变化触及该服务目录，仍需实现专门冲突语义
5. 迁移 UI 仍是基础入口；任务详情页已提供 `awaiting_confirmation` 下的通过/拒绝入口，但整体确认体验和目标节点选择体验仍需增强
6. 整个 migrate 流程需要近生产环境端到端测试；当前重点验证源节点 stop/backup、目标节点 restore/start、确认后 dns/persist_repo
7. restore 覆盖语义、数据库导入语义、失败后的残留状态处理需要重构并补测试
8. 任务查询与筛选已把 `awaiting_confirmation` 视为一等状态；仍需继续打磨相关 UI 呈现和操作流

---

## 目标 API 结构

### ServiceQueryService
- [x] `ListServices` - 返回 `ServiceSummary[]`，含 instance_count/running_count/target_node_count
- [x] `GetService` - 返回 `ServiceDetail`，含 `nodes[]` 和 `instances[]`
- [x] `GetServiceTasks`
- [x] `GetServiceBackups`

### ServiceCommandService
- [x] `UpdateServiceTargetNodes` - 定向改写 `composia-meta.yaml.nodes`
- [x] `RunServiceAction` - 支持 `node_ids[]` 数组进行 fan-out，`data_names[]` 用于 backup
- [x] `UpdateServiceDNS` - 通过 `RunServiceAction` + `SERVICE_ACTION_DNS_UPDATE` 实现
- [x] `MigrateService(source_node_id, target_node_id)` - 迁移基础能力已实现

### TaskService
- [x] `ListTasks`
- [x] `GetTask`
- [x] `TailTaskLogs`
- [x] `RunTaskAgain`
- [x] `ResolveTaskConfirmation(task_id, decision, comment)` - 仅用于推进 `awaiting_confirmation` 中的已有 task，不负责创建新 migrate

### ServiceInstanceService
- [x] `ListServiceInstances` - 返回 `ServiceInstanceSummary[]`
- [x] `GetServiceInstance(service_name, node_id)` - 返回 `ServiceInstanceDetail`，含 `containers[]`
- [x] `RunServiceInstanceAction` - 针对单个 instance 执行动作

### ContainerService
- [x] `RunContainerAction(node_id, container_id, action)` - start/stop/restart
- [x] `GetContainerLogs(node_id, container_id, tail, timestamps)`
- [x] `OpenContainerExec(node_id, container_id, command, rows, cols)` - 返回 websocket path

### NodeQueryService
- [x] `ListNodes` - 返回 `NodeSummary[]`
- [x] `GetNode`
- [x] `GetNodeTasks`
- [x] `GetNodeDockerStats`

### NodeMaintenanceService
- [x] `PruneNodeDocker(node_id)`
- [x] `ReloadNodeCaddy(node_id)`
- [x] `SyncNodeCaddyFiles(node_id)`
- [x] `ForgetNodeRustic(...)`
- [x] `PruneNodeRustic(...)`

### DockerQueryService
- [x] `ListNodeContainers(node_id)`
- [x] `InspectNodeContainer(node_id, container_id)`
- [x] `ListNodeNetworks(node_id)`
- [x] `InspectNodeNetwork(node_id, network_id)`
- [x] `ListNodeVolumes(node_id)`
- [x] `InspectNodeVolume(node_id, volume_name)`
- [x] `ListNodeImages(node_id)`
- [x] `InspectNodeImage(node_id, image_id)`

### RepoQueryService
- [x] `GetRepoHead`
- [x] `ListRepoFiles`
- [x] `GetRepoFile`
- [x] `ListRepoCommits`
- [x] `ValidateRepo`

### RepoCommandService
- [x] `UpdateRepoFile`
- [x] `CreateRepoDirectory`
- [x] `MoveRepoPath`
- [x] `DeleteRepoPath`
- [x] `SyncRepo`

---

## 待实现的 CLI 命令（v1）

根据 multi-node 模型，CLI 应支持：

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

当前状态：CLI 仍处于极早期阶段；当前仅实现 `composia controller` 和 `composia agent` 两个入口，尚未实现面向用户的 `service`、`instance`、`container`、`task`、`backup`、`node`、`repo`、`secret`、`system` 等命令面。

---

## 推荐的直接下一步

1. ~~完成 repo schema 和 store schema 的 multi-node 重构~~ - 已完成
2. ~~完成 `ServiceInstanceService` 和 service fan-out 任务创建~~ - 已完成
3. ~~完成 agent 的 instance 状态上报改造~~ - 已完成
4. ~~完成 `ContainerService` 的 logs/start/stop/restart/exec 基础接口~~ - 已完成
5. ~~实现 `ReloadNodeCaddy` API 和 UI 入口~~ - 已完成
6. ~~实现 `caddy_reload` 任务行为~~ - 已完成
7. 完善 migrate 的 `persist_repo` 冲突处理，以及确认后的异常/回滚语义（Phase 9）
8. 把 restore 从“已有 API/UI 入口”推进到“真实环境验证完成”：按 strategy 跑通回归并收口错误语义
9. 优先补齐真实环境测试：`migrate`、`ForgetNodeRustic`、`PruneNodeRustic`、`restore`
10. 按 strategy 重构 backup/restore：`files.copy`、`files.tar_after_stop`、`database.pgdumpall`、`files.untar`、`database.pgimport`
11. CLI 命令面实现：`service`、`instance`、`container`、`task`、`backup`、`node`、`repo`、`secret`、`system`
12. Web UI 增强：改进的 terminal 组件、迁移确认体验、restore 交互细节

## 基于当前代码的补充结论

以下内容已通过实际代码确认，旧文档中的对应描述已经过时：

- `awaiting_confirmation` 已不是占位状态；controller、task API、测试和 Web UI 操作入口都已接通。
- 定时任务执行已实现，包含 service backup 调度以及 rustic forget/prune 调度。
- Web UI 已支持把 `awaiting_confirmation` 作为任务状态筛选项。
- CodeMirror 基础检查已落地，至少包含 `.env` 与 compose YAML lint。
- 备份记录查询 API 已支持 `ListBackups` 和 `GetBackup`，Web 端已有备份详情页。
- restore 已有独立 API 和 Web 入口，但真实环境验证和失败语义仍需补强。
- 近生产环境反馈已确认 `rustic init` 跑通、Caddy 功能正常；验证重点收敛到 `restore`、`migrate`、`ForgetNodeRustic`、`PruneNodeRustic`。

---

## 架构要点提醒

- 不要再扩展旧的单节点 service 语义。
- `Service` 不是部署实例；部署实例必须建模为 `ServiceInstance`。
- `Container` 是一等对象，不只是 node 的一个附属列表。
- 仅按文档化的工作流实现 `migrate`，不是快捷变体。
- 保持 controller 作为持久状态所有者，agent 作为执行端。
- 所有 repo 写入必须经过 repo lock 串行化。
- 所有 instance 和 container 动作都必须保留 `node_id` 这一维。
