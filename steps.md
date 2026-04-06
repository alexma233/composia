# Composia Implementation Steps

This document turns `plan.md` into a practical execution order for the current repository state.

## Source of Truth

- `plan.md` 是产品和架构的真相源。
- `steps.md` 只是一个执行排序文档。
- 如果 `steps.md` 和 `plan.md` 有冲突，以 `plan.md` 为准。
- 不要把 scaffold、占位符或不完整的 API 当作"已完成"。
- 在增加新功能之前，先消除当前的架构漂移。

## 当前代码库状态

代码库已超越初始 scaffold 阶段。

### 已实现或大部分实现

**核心基础设施：**
- `composia controller` 和 `composia agent` 子命令已存在。
- `config.yaml` 加载和验证已实现（controller 和 agent 模型）。
- Controller 启动初始化本地目录和打开 SQLite。
- SQLite schema 存在：`nodes`、`services`、`tasks`、`task_steps`、`backups`、repo sync state。
- Controller-agent ConnectRPC 连接已存在：heartbeat、long-poll task pull、bundle download、task state、step state、log upload、backup reporting、service runtime reporting、Docker stats reporting。
- Agent heartbeat 工作正常，节点状态已持久化。
- 服务 repo 扫描和 `composia-meta.yaml` 验证已存在。

**Controller Public API（已完成）：**
- `ServiceService`：ListServices、GetService、GetServiceTasks、GetServiceBackups、DeployService、UpdateService、StopService、RestartService、BackupService、UpdateServiceDNS
- `TaskService`：ListTasks、GetTask、TailTaskLogs、RunTaskAgain
- `BackupRecordService`：ListBackups、GetBackup
- `NodeService`：ListNodes、GetNode、GetNodeTasks、GetNodeDockerStats、PruneNodeDocker、ListNodeContainers/Networks/Volumes/Images、InspectNodeContainer/Network/Volume/Image
- `RepoService`：GetRepoHead、ListRepoFiles、GetRepoFile、UpdateRepoFile、ListRepoCommits、SyncRepo
- `SecretService`：GetServiceSecretEnv、UpdateServiceSecretEnv
- `SystemService`：GetSystemStatus、GetCurrentConfig

**Agent 任务执行（已完成）：**
- `deploy`：完整实现（bundle download、render、compose-up steps）
- `update`：完整实现（pull + compose-up steps）
- `stop`：完整实现（download bundle、compose-down）
- `restart`：完整实现（compose-down + compose-up）
- `backup`：完整实现（rustic、files.copy、files.tar_after_stop、database.pgdumpall）
- `prune`：完整实现（targets: all、containers、networks、images、volumes、builder）
- `dns_update`：Controller 端实现（Cloudflare 集成）

**其他已实现：**
- Repo Git 操作（lock handling、commit、fetch+fast-forward、push）
- age-based secret 加密/解密
- 自动 repo pull（使用 `pull_interval`）
- Web UI 连接到真实 controller API
- Docker 浏览 API（containers、networks、images、volumes）

### 尚未实现

**缺失的 API（proto 中）：**
- `ServiceService.MigrateService` - 未实现
- `NodeService.ReloadNodeCaddy` - 未实现

**缺失的任务执行：**
- `migrate` 任务类型 - 未实现
- `caddy_reload` 任务类型 - 未实现

**缺失的备份/恢复功能：**
- restore 任务类型 - 未实现
- `RestoreService` API - 未实现
- 迁移复用备份数据的工作流 - 不完整

**缺失的功能：**
- CLI 命令面（`composia-cli.yaml` 处理） - 未实现
- 定时任务调度（cron/scheduler） - 未实现
- `auto_deploy` 配置选项 - 已文档化但未连接到任务创建
- CodeMirror 6 网页编辑器 - 未实现（只有基础 textareas）
- AI/MCP/Skills - 未实现（v1 不计划）

---

## 执行规则

1. 保持实现与 `plan.md` 一致，即使需要收紧或替换现有占位符行为。
2. 优先完成已开始的基础工作，再添加更多 API 或 UI 页面。
3. 不要添加改变任务语义、repo 语义或 controller-agent 职责的新行为，除非 `plan.md` 已定义。
4. 将迁移、备份、DNS、secrets 和 repo 写入视为架构敏感工作，必须匹配文档化的 v1 语义，而不是快捷变体。

---

## Phase 1: 消除架构漂移

**状态：大部分完成**

目标：在扩展功能之前，使当前后端与 `plan.md` 中描述的 controller-agent 契约匹配。

已完成：
1. `controller` 是持久状态所有者和任务调度器。
2. `agent` 是通过 `PullNextTask` 和 `GetServiceBundle` 的执行端。
3. 任务日志从 agent 流式上传到 controller 并持久化。
4. Agent 报告服务运行时状态。

待完成：
- 无

---

## Phase 2: 完成任务基础

**状态：已完成**

目标：使现有任务系统可靠且严格符合文档化的 v1 任务模型。

已完成：
1. 每个任务绑定到特定 `repo_revision`。
2. 全局串行执行语义。
3. `pending`、`running`、`awaiting_confirmation` 和终态规则。
4. 任务步骤摘要和每任务日志在 `controller.log_dir`。
5. `running` 任务的重启恢复行为。
6. 服务级冲突检查与 repo 写入冲突规则一致。

待完成：
- `awaiting_confirmation` 保留用于未来真实的迁移工作流。

---

## Phase 3: 稳定第一个真实服务操作

**状态：已完成**

目标：在添加更广泛工作流之前，完成已开始 day-1 服务操作。

已完成：
1. `deploy` 是第一个完全支持的端到端任务。
2. `update`、`stop`、`restart` 任务步骤和运行时效果符合计划。
3. Agent 报告的运行时状态。
4. 任务日志流式回传。

---

## Phase 4: 添加安全的 desired-state Repo 写入

**状态：大部分完成**

目标：让 controller 完全按照文档管理 Git-backed desired state 变更。

已完成：
1. Repo lock 处理、验证、服务冲突检查和本地 commit 创建。
2. 可选远程同步行为、push 报告和 repo sync 状态。
3. `RepoService.SyncRepo` 实现。
4. `GetRepoHead` 返回同步相关状态。
5. `GetCurrentConfig` API 已实现。

待完成：
- `auto_deploy` 选项（auto-deploy after repo changes）已文档化但未实现。

---

## Phase 5: 添加 Secret 处理

**状态：已完成**

目标：实现选定的 age-based secrets 模型，不在 `controller.repo_dir` 留下明文。

已完成：
1. Controller 端解密和重新加密与配置的 age 设置一致。
2. `SecretService.GetServiceSecretEnv` 和 `UpdateServiceSecretEnv` 与普通 repo 写入的 repo lock 和冲突语义一致。
3. 解密后的运行时文件只包含在 agent bundle 中，不持久化在 controller Git 工作树。
4. Git remote-sync 语义扩展到 secret 写入。

---

## Phase 6: 替换占位符备份行为

**状态：进行中**

目标：将当前备份 scaffold 转变为第一个真实数据保护工作流。

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

---

## Phase 7: 添加 Read-Write Web UI

**状态：进行中**

目标：保留现有真实 controller UI，完成缺失的 controller 交互，并继续将其收紧为密集操作控制台。

已完成：
- Dashboard、services、nodes、tasks、backups 页面连接到真实 API
- 服务动作（deploy、update、stop、restart、backup）
- 任务日志 tailing
- Repo 编辑和 sync 状态显示
- Secret 编辑
- Docker 浏览（containers、networks、images、volumes）

待完成：
1. `MigrateService` UI 入口
2. `ReloadNodeCaddy` UI 入口
3. 任务 rerun UI 的完整控件
4. Repo sync 手动触发按钮（push）
5. CodeMirror 6 编辑器替换基础 textareas
6. 备份详情页的 restore 入口（未来）

---

## Phase 8: 添加 DNS、Caddy 和节点操作

**状态：进行中**

目标：在 repo 写入和基础服务流稳定后，实现文档化的 day-2 操作动作。

已完成：
1. `dns_update` 任务行为 - controller 端 Cloudflare 集成
2. `prune` 任务行为 - all/containers/images/networks/volumes/builder targets
3. `PruneNodeDocker` API

待完成：
1. `caddy_reload` 任务行为 - 未实现
   - 需要在 agent 添加 Caddyfile 验证和 reload 执行逻辑
   - 需要 `ReloadNodeCaddy` API
2. `ReloadNodeCaddy` API - 未实现

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
   - 目标节点服务启动
   - 刷新目标节点 Caddy 生成目录
   - 重建源节点 Caddy 生成目录（移除旧服务片段）
   - DNS 更新
   - `persist_repo` 阶段：修改 `composia-meta.yaml.node` 并 commit/push
3. `awaiting_confirmation` 状态用于迁移后人工验证和 repo 对账
4. 迁移冲突处理：当 `persist_repo` 时 `HEAD` 已变化但变化触及该服务目录

---

## 待实现的 API（来自 plan.md v1 规范）

### ServiceService
- [ ] `MigrateService(target_node_id)` - 创建迁移任务，接收目标 node_id

### NodeService  
- [ ] `ReloadNodeCaddy(node_id)` - 创建 caddy_reload 任务

---

## 待实现的备份/恢复功能

### 备份策略（已实现）
- [x] `files.copy`
- [x] `files.tar_after_stop`
- [x] `database.pgdumpall`

### 备份策略（未实现）
- [ ] 其他 provider（通过 PR 扩展）

### 恢复策略（未实现）
- [ ] `database.pgimport`
- [ ] `files.untar`
- [ ] `files.copy` (restore)

---

## 待实现的 CLI 命令（v1）

根据 plan.md，CLI 应支持：

```
service list/get/deploy/stop/restart/update/backup/migrate/logs
task list/get/run-again/logs
backup list/get
node list/get/tasks/reload-caddy/prune
repo head/files/get/update/history/sync
secret get/update
system status
caddy reload
dns update
```

当前状态：CLI 配置文件格式已定义但 CLI 命令面未实现。

---

## 待实现的 Web UI 功能

### 页面（已完成）
- [x] 服务列表
- [x] 服务详情
- [x] 节点列表
- [x] 节点详情 + Docker 浏览
- [x] 备份状态
- [x] 任务历史
- [x] 设置页（配置显示、repo sync 状态）
- [x] Repo 文件浏览和编辑
- [x] Secret 编辑

### 页面（待实现）
- [ ] 文档页
- [ ] CodeMirror 6 编辑器（替换基础 textareas）

### UI 操作（待实现）
- [ ] MigrateService 入口
- [ ] ReloadNodeCaddy 入口
- [ ] 手动 push trigger（repo sync 页面）
- [ ] 备份 restore 入口（未来）

---

## 推荐的直接下一步

Phase 2、3 和 4 大部分已完成。下一个里程碑专注于完成剩余差距并为更高级工作流做准备：

1. **添加 Caddy 管理**
   - 实现 `caddy_reload` 任务
   - 实现 `ReloadNodeCaddy` API
   - 在 agent 添加 Caddyfile 验证和 reload 执行

2. **实现迁移（Phase 9）**
   - 添加 `MigrateService` API
   - 添加 `migrate` 任务执行逻辑
   - 实现 `awaiting_confirmation` 流

3. **完成备份恢复功能**
   - 实现 `database.pgimport` 和 `files.untar` restore 策略
   - 添加 restore 任务类型
   - 添加备份 restore UI

4. **实现 auto_deploy**
   - 将 `auto_deploy` 配置选项连接到任务创建

5. **实现 CLI 命令面**
   - 处理 `composia-cli.yaml` 配置
   - 实现核心 CLI 子命令

6. **完善 Web UI**
   - CodeMirror 6 编辑器
   - 迁移和 caddy reload 入口
   - 手动 push trigger

---

## 架构要点提醒

- 不要扩展 DNS、调度或更多 UI 区域，直到这些对齐项目完成。
- 仅按文档化的工作流实现 `migrate`，不是快捷变体。
- 保持 controller 作为持久状态所有者，agent 作为执行端。
- 所有 repo 写入必须经过 repo lock 串行化。
- 服务级冲突规则：`pending`、`running` 或 `awaiting_confirmation` 任务存在时，不允许创建该服务的新任务，也不允许写入触及该服务目录的 repo 文件。
