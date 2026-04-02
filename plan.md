### 1. 项目目标（一句话定位）

**Composia = 服务声明 + 部署控制面 + 运行节点执行面**

核心目标：做一个**以服务为核心**的自建服务管理器，支持单节点/多节点，重点解决实际自建运维中的备份、迁移、更新、监控等痛点，而不是做一个普通的 Docker Compose GUI。

### 2. 核心设计原则（已全部确定）

- **以服务为主**：所有操作（部署、备份、迁移、监控）都围绕“服务”（如 stalwart、vaultwarden）而不是文件类型。
- **一个服务只部署在一个节点**：v1 不支持多副本/高可用（后续 v2 再单独设计）。
- **单主控制面**：永远只有 1 个 active main + N 个 agent，不做 active-active。
- **同一二进制不同角色**：`composia` 一个程序，通过启动参数切换 `main` / `agent` 模式。
- **Git 只存声明**：Git 负责服务定义、meta.yaml、模板、历史；运行时状态全部进 SQLite。
- **main 持全量，agent 只持本地**：agent 只保存自己需要运行的服务包，减少暴露和同步量。
- **Caddy 每个节点独立**：每个 agent 节点独立运行一个 Caddy 实例，只提供一个 startup 模板（cf 指 Cloudflare Origin 模式），用户自己手写配置。
- **备份默认内置 rustic**：其他 provider 通过 PR 扩展。
- **前端**：SvelteKit + shadcn-svelte（Tailwind）。
- **网页编辑器**：CodeMirror 6（支持 YAML 高亮 + 基本 lint，可后续加自定义关键词检查）。
- **Mobile Support**：Web UI 响应式（手机浏览器可用），不需要 PWA。
- **Image update**：同时支持手动触发 + meta.yaml 配置自动 schedule。
- **Podman**：v1 只支持 Docker，provider 抽象已留好扩展位。
- **i18n**：只留接口，主要英文，只做 Web UI（CLI 不需要）。
- **DNS**：自动帮服务更新 DNS 记录（v1 只支持 Cloudflare）。
- **权限**：单用户模式。
- **数据库**：SQLite（keep it simple）。
- **后端语言**：Go。

---

### 3. 总体架构（已确定）

- **Main**（控制面）：Git 拉取、配置渲染、任务调度、备份编排、Caddy 片段分发、状态数据库、API、Web UI、CLI 入口。
- **Agent**（执行面）：接收 main 下发的服务包、本地落盘、执行 docker compose up/down/pull、执行备份 hook、上报状态。
- **通信方式**：ConnectRPC + 预共享 token 认证

---

### 4. 目录结构（推荐结构，已基本确定）

```text
composia/                  # 程序安装目录（main 和 agent 都一样）
  config.yaml
  .secrets.env             # 运行时解密后的 secrets（不进 Git）
  state/composia.db
  logs/
    tasks/

repo/                      # Git 仓库根目录（main 全量，agent 只本地服务）
  .secrets.enc.yaml        # 加密后的 secrets 文件
  stalwart/
    docker-compose.yaml
    .env.template
    composia-meta.yaml
    config/
    site_config.caddy
  vaultwarden/
    ...
  caddy/                     # 全局 Caddy 模板（每个节点独立使用）
    docker-compose.yaml
    config/
      Caddyfile
      snippet/
      site/
```

agent 本地只会保留自己需要运行的服务 + caddy。

---

### 5. 服务定义模型（meta.yaml）（v1 范围已确定）

说明：`meta.yaml` 只描述 `composia` 自己需要的编排信息；资源限制、依赖、hooks 等尽量继续交给 `docker compose` 表达，不在这里重复造一层 DSL。

已确定字段示例：

```yaml
name: vaultwarden
node: node-2
project_name: vaultwarden
enabled: true

network:
  caddy:
    enabled: true
    source: ./site_config.caddy
  dns:
    provider: cloudflare
    hostname: vaultwarden.alexma.top
    record_type: A
    value: 203.0.113.10

update:
  strategy: pull_and_recreate
  backup_before_update: true
  schedule: "0 4 * * *"

backup:
  database:
    enabled: true
    provider: rustic
    strategy: pgdump
    schedule: "0 2 * * *" # 每天凌晨 2 点 (cron)
    retain: 30d
    pg_addr: http://pg:8080
  config:
    enabled: true
    provider: rustic
    strategy: copy
    schedule: "0 2 * * *" # 每天凌晨 2 点 (cron)
    retain: 30d
    include:
      - ./config
      - vaultwarden_config # Docker Volumes
```

- 服务运行状态以 `docker compose` / 容器状态为准。
- `composia` 不内建主动探测、业务健康检查或自动回滚逻辑。

---

### 6. 备份系统（已确定大部分）

- 统一抽象（pre-hook / dump / copy / snapshot / retention）。
- 默认 provider：rustic（内置），其他后续 PR。
- PostgreSQL 默认用 pg_dump / pg_dumpall（不强制 pg_basebackup）。
- 备份完成后**必须上报 main** 存入 SQLite。
- 迁移时**强制备份**（即使 meta.yaml 里关闭备份）。
- `v1` 不内建 restore 工作流，默认接受人工恢复。

---

### 7. 服务迁移流程（已确定）

1. 修改 meta.yaml 中的 node。
2. main 自动备份旧节点。
3. 停止旧节点服务。
4. rsync 数据包到新节点。
5. 新节点启动。
6. 更新 Caddy 配置 + DNS（Cloudflare）。
7. 用户人工验证服务可用性；失败后由用户人工处理。

---

### 8. Secrets 管理

- 加密后进 Git（`.secrets.enc.yaml`，用户自行用 `age` / `sops` 加密）。
- `main` 负责解密、渲染并下发到目标 `agent`。
- `agent` 不承担解密职责，只接收自身需要的运行时 secrets。

---

### 9. AI 助手 / MCP / Skills

`v1` 不接模型能力，只保留文档入口或操作说明页。

---

### 10. 节点注册与 Main 高可用

- `v1` 采用完全手动注册。
- `main` 手动维护节点列表。
- `agent` 在 `config.yaml` 中手动配置 `main` 地址、节点 ID、预共享 token。
- 不做 `main` 高可用；`main` 挂了就接受人工切换和人工恢复。

### 11. Web UI 页面

核心页面：服务列表 / 服务详情 / 节点列表 / 备份状态 / 任务历史 / 文档页 / 设置页。

`v1` 页面上必须可直接完成：

- 在线编辑仓库文件（`docker-compose.yaml`、`.env.template`、`composia-meta.yaml`、Caddy 片段）。
- 部署控制（部署、停止、重启、更新、迁移）。
- 节点运维（节点状态、磁盘、Docker 信息、最近心跳）。

---

### 12. CLI 命令（v1）

已确认核心：`service list/deploy/backup/migrate/logs`、`node list`、`prune`、`status`、`caddy reload`、`dns update`。

CLI 定位为运维入口，覆盖 SSH / 脚本化场景的常用操作。

---

### 13. 其他已确定但细节待细化的

- Git 同步：main 检测到更新后**自动拉取**
- Git 更新失败：仓库内容非法或渲染失败时，拒绝应用新版本，保留当前运行状态
- 失败恢复：main 挂了就接受“手动操作 compose”的现实
- 任务执行：默认串行保守，不做复杂并发调度
- Agent 离线：相关任务直接报错，不做任务队列
- 状态数据库：SQLite 只存结构化状态（节点/服务/任务/备份/心跳等）
- 任务日志：详细执行日志单独落文件，SQLite 不承载大段日志内容
