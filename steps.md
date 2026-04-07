# Composia Implementation Steps

This document turns `plan.md` into a practical execution order for the current repository state.

## Source of Truth

- `plan.md` 是产品和架构的真相源。
- `steps.md` 只是一个执行排序文档。
- 如果 `steps.md` 和 `plan.md` 有冲突，以 `plan.md` 为准。
- 不要把 scaffold、占位符或不完整的 API 当作"已完成"。
- 在增加新功能之前，先消除当前的架构漂移。

## 当前重构结论

从本次重构开始，仓库正式采用 multi-node 架构。

- `Service` 表示逻辑服务定义，不再表示某个单节点上的运行实体。
- `ServiceInstance` 表示一个服务在某个节点上的实际部署实例，主键为 `(service_name, node_id)`。
- `Container` 表示某个节点上的 Docker 容器，并通过 compose labels 关联到 `ServiceInstance`。
- `Node` 保持为 agent 和 Docker 宿主资源的管理对象。

这意味着旧的"一个 service 对应一个 node"假设全部废弃。凡是公开 API、数据库结构、repo schema、任务模型、Web UI 文案仍然携带该假设的地方，都必须被替换。

## 当前代码库状态

代码库已超越初始 scaffold 阶段，但当前实现仍然存在明显的单节点 service 假设。

### 已实现或大部分实现

**核心基础设施：**
- `composia controller` 和 `composia agent` 子命令已存在。
- `config.yaml` 加载和验证已实现（controller 和 agent 模型）。
- Controller 启动初始化本地目录和打开 SQLite。
- Controller-agent ConnectRPC 基础链路已存在：heartbeat、long-poll task pull、bundle download、task state、step state、log upload、backup reporting、Docker stats reporting。
- Agent heartbeat 工作正常，节点状态已持久化。
- 服务 repo 扫描和 `composia-meta.yaml` 验证已存在。
- Docker 浏览 API（containers、networks、images、volumes）已存在，但仍然挂在 node 维度上。

**Controller Public API（已存在，但需要重构语义）：**
- `ServiceService`
- `TaskService`
- `BackupRecordService`
- `NodeService`
- `RepoService`
- `SecretService`
- `SystemService`

**Agent 任务执行（已完成）：**
- `deploy`：完整实现（bundle download、render、compose-up steps）
- `update`：完整实现（pull + compose-up steps）
- `stop`：完整实现（download bundle、compose-down）
- `restart`：完整实现（compose-down + compose-up）
- `backup`：完整实现（rustic、files.copy、files.tar_after_stop、database.pgdumpall）
- `prune`：完整实现（targets: all、containers、networks、images、volumes、builder）
- `dns_update`：Controller 端实现（Cloudflare 集成）

### 当前的架构漂移

以下内容仍然错误地假设 service 只有一个 node：

1. `composia-meta.yaml` 只支持 `node`。
2. `repo.Service` 只保留单个 `Node`。
3. `services` 表把运行时状态直接挂在 service 上。
4. agent 上报运行时状态时只传 `service_name`，不传 `node_id`。
5. `DeployService/UpdateService/StopService/RestartService/BackupService` 一次只创建一条 task。
6. `GetService` 只返回单个 `node`。
7. Web UI 仍然把 service 当作单节点部署对象。

### 必须新增的一等对象

1. `Service`
2. `ServiceInstance`
3. `Container`
4. `Node`

---

## 执行规则

1. 保持实现与 `plan.md` 一致，即使需要收紧或替换现有占位符行为。
2. 优先完成 multi-node 主干，不再向旧的单节点 service 语义继续叠加功能。
3. 不要把容器操作继续塞进单节点 service API；应改为资源清晰的 instance/container API。
4. 将迁移、备份、DNS、secrets 和 repo 写入视为架构敏感工作，必须匹配文档化的 v1 语义，而不是快捷变体。

---

## Phase 1: 切换到 Multi-Node 语义

**状态：进行中**

目标：在扩展 Caddy 和容器操作之前，把当前后端从"单 service 对应单 node"切换到"service 定义 + service instance"契约。

待完成：
1. `composia-meta.yaml` 从 `node` 切换为 `nodes[]`。
2. `repo.Service` 改为持有 `TargetNodes`。
3. repo 验证、扫描、查找逻辑全部切换为 multi-node。
4. `services` 表仅表示逻辑服务；新增 `service_instances` 表表示实例。
5. 所有 service 运行时状态从 instance 聚合得到。
6. agent 运行时上报切换到 `service_name + node_id`。

---

## Phase 2: 完成 Multi-Target 任务基础

**状态：进行中**

目标：使任务系统可靠且严格符合 multi-node 的 v1 任务模型。

待完成：
1. service 动作改为 fan-out 到多个 instance task。
2. service 级冲突检查改为 service 和 instance 分层检查。
3. 任务详情保留 `service_name` 和 `node_id`，明确指向实例级目标。
4. `RunTaskAgain` 语义与 instance 目标保持一致。
5. `awaiting_confirmation` 保留用于未来真实的迁移工作流。

---

## Phase 3: 稳定第一个真实 ServiceInstance 操作

**状态：进行中**

目标：在添加更广泛工作流之前，完成已开始的 day-1 instance 操作。

待完成：
1. `deploy`、`update`、`stop`、`restart` 对单个 instance 的行为必须稳定。
2. service 级动作只作为批量入口，内部展开为 instance 动作。
3. Agent 报告的是 instance 运行时状态，而不是全局 service 状态。
4. 任务日志继续流式回传。

---

## Phase 4: 添加安全的 desired-state Repo 写入

**状态：进行中**

目标：让 controller 完全按照文档管理 Git-backed desired state 变更，并与 multi-node repo 语义对齐。

待完成：
1. Repo lock 处理、验证、服务冲突检查和本地 commit 创建继续有效。
2. `composia-meta.yaml.nodes` 的改写逻辑与迁移工作流对齐。
3. 可选远程同步行为、push 报告和 repo sync 状态继续工作。
4. `auto_deploy` 选项（auto-deploy after repo changes）后续接到 instance 扇出任务创建。

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

**状态：进行中**

目标：将当前备份 scaffold 转变为第一个真实数据保护工作流，并明确其 instance 语义。

已完成：
- 备份执行使用 rustic provider、files.copy、files.tar_after_stop、database.pgdumpall
- 备份记录持久化到 SQLite
- 备份查询 API 稳定

待完成：
1. restore 任务执行 - 未实现
2. 迁移复用备份数据的工作流 - 不完整
3. `database.pgimport` restore 策略 - 未实现
4. `files.untar` restore 策略 - 未实现
5. 定时备份执行 - 未实现
6. service 级 backup 需要明确是对全部实例、单实例还是仅支持单实例入口

---

## Phase 7: 添加 Multi-Node Read-Write Web UI

**状态：进行中**

目标：保留现有真实 controller UI，但重切成 `service / service instance / container / node` 四层模型。

已完成：
- Dashboard、services、nodes、tasks、backups 页面连接到真实 API
- 任务日志 tailing
- Repo 编辑和 sync 状态显示
- Secret 编辑
- Docker 浏览（containers、networks、images、volumes）

待完成：
1. Service 详情页显示 instances 列表和聚合状态。
2. ServiceInstance 详情页。
3. Container 详情页增加日志、start/stop/restart、exec terminal。
4. `MigrateService` UI 入口。
5. `ReloadNodeCaddy` UI 入口。
6. 任务 rerun UI 的完整控件。
7. Repo sync 手动触发按钮（push）。
8. CodeMirror 6 编辑器替换基础 textareas。
9. 备份详情页的 restore 入口（未来）。

---

## Phase 8: 添加 DNS、Caddy 和容器操作

**状态：进行中**

目标：在 repo 写入和基础服务流稳定后，实现文档化的 day-2 操作动作，并将 Caddy 放在 multi-node service model 上。

已完成：
1. `dns_update` 任务行为 - controller 端 Cloudflare 集成
2. `prune` 任务行为 - all/containers/images/networks/volumes/builder targets
3. `PruneNodeDocker` API

待完成：
1. `caddy_reload` 任务行为 - 未实现
2. `ReloadNodeCaddy` API - 未实现
3. `caddy` 作为多节点 service 的实例扇出语义 - 未实现
4. Container logs API - 未实现
5. Container start/stop/restart API - 未实现
6. Container exec session API - 未实现

---

## Phase 9: 最后添加迁移

**状态：待开始**

目标：仅在整个平台语义稳定后，实现最复杂的 v1 工作流。

待实现：
1. `MigrateService(target_node_id)` RPC
2. `migrate` 任务类型，包括：
   - 源节点数据导出（使用 `migrate.data[]` 引用的 `backup` 定义）
   - 数据和运行时文件传输到目标节点
   - 目标节点数据恢复（使用 `restore` 定义）
   - 目标节点 service instance 启动
   - 刷新目标节点 Caddy 生成目录
   - 重建源节点 Caddy 生成目录（移除旧服务片段）
   - DNS 更新
   - `persist_repo` 阶段：修改 `composia-meta.yaml.nodes` 并 commit/push
3. `awaiting_confirmation` 状态用于迁移后人工验证和 repo 对账
4. 迁移冲突处理：当 `persist_repo` 时 `HEAD` 已变化但变化触及该服务目录

---

## 目标 API 结构

### ServiceService
- [ ] `ListServices`
- [ ] `GetService`
- [ ] `GetServiceTasks`
- [ ] `GetServiceBackups`
- [ ] `DeployService`
- [ ] `UpdateService`
- [ ] `StopService`
- [ ] `RestartService`
- [ ] `BackupService`
- [ ] `UpdateServiceDNS`
- [ ] `MigrateService(target_node_id)`

### ServiceInstanceService
- [ ] `ListServiceInstances`
- [ ] `GetServiceInstance(service_name, node_id)`
- [ ] `DeployServiceInstance(service_name, node_id)`
- [ ] `UpdateServiceInstance(service_name, node_id)`
- [ ] `StopServiceInstance(service_name, node_id)`
- [ ] `RestartServiceInstance(service_name, node_id)`
- [ ] `BackupServiceInstance(service_name, node_id)`
- [ ] `ListServiceInstanceContainers(service_name, node_id)`

### ContainerService
- [ ] `ListContainers`
- [ ] `GetContainer(node_id, container_id)`
- [ ] `TailContainerLogs(node_id, container_id)`
- [ ] `StartContainer(node_id, container_id)`
- [ ] `StopContainer(node_id, container_id)`
- [ ] `RestartContainer(node_id, container_id)`
- [ ] `CreateContainerExecSession(node_id, container_id)`

### NodeService
- [ ] `ListNodes`
- [ ] `GetNode`
- [ ] `GetNodeTasks`
- [ ] `GetNodeDockerStats`
- [ ] `PruneNodeDocker(node_id)`
- [ ] `ReloadNodeCaddy(node_id)`

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

当前状态：CLI 配置文件格式已定义但 CLI 命令面未实现。

---

## 推荐的直接下一步

1. 完成 repo schema 和 store schema 的 multi-node 重构。
2. 完成 `ServiceInstanceService` 和 service fan-out 任务创建。
3. 完成 agent 的 instance 状态上报改造。
4. 完成 `ContainerService` 的 logs/start/stop/restart/exec 基础接口。
5. 将 Caddy 接到多节点 service model 上。

---

## 架构要点提醒

- 不要再扩展旧的单节点 service 语义。
- `Service` 不是部署实例；部署实例必须建模为 `ServiceInstance`。
- `Container` 是一等对象，不只是 node 的一个附属列表。
- 仅按文档化的工作流实现 `migrate`，不是快捷变体。
- 保持 controller 作为持久状态所有者，agent 作为执行端。
- 所有 repo 写入必须经过 repo lock 串行化。
- 所有 instance 和 container 动作都必须保留 `node_id` 这一维。
