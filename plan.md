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
  state/composia.db
  logs/
    tasks/

repo/                      # Git 仓库根目录（main 全量，agent 只本地服务）
  stalwart/
    docker-compose.yaml
    .env
    .secret.env.enc
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
agent 落盘后，服务目录中会有解密后的 `.secret.env` 供 `docker compose` 使用，该文件不进 Git。

---

### 4.1 运行配置（config.yaml）（v1 正式规范）

设计约束：

- `config.yaml` 是每台机器本地独立维护的配置文件，不共享同一份物理文件。
- 启动角色由命令决定：`composia main` / `composia agent`。
- 配置结构按角色分区：`main:` 和 `agent:`。
- 实际部署时，`main` 主机通常只写 `main:`，`agent` 主机通常只写 `agent:`；未使用的 section 可省略。

#### `main` 配置

字段定义：

- `listen_addr`：必填；本机监听地址；用于绑定 API / Web 服务。
- `public_addr`：必填；完整 URL；供 agent 和 CLI 连接 main。
- `repo_dir`：必填；本地 Git 工作目录。
- `state_dir`：必填；SQLite 和其他本地状态文件目录。
- `log_dir`：必填；任务详细日志目录。
- `git`：可选；不写时表示只使用现有本地 `repo_dir`，不托管 Git 拉取。
- `nodes`：必填；节点清单；`main` 自己也必须包含在内。
- `cli_tokens`：可选；CLI 访问 main 的令牌列表；`v1` 先手写在配置中。

`main.git`：

- 如果 `remote_url` 不存在，则视为本地模式，`composia` 只读取 `repo_dir`。
- 如果 `remote_url` 存在，则视为托管拉取模式。
- `remote_url`：可选；存在时启用托管拉取。
- `branch`：可选；默认 remote HEAD。
- `pull_interval`：在托管拉取模式下必填；使用 Go duration 格式，如 `30s`、`5m`。
- `auth.token_file`：可选；Git 认证 token 文件路径；为未来扩展 SSH/key 留结构空间。

`main.nodes[]`：

- `id`：必填；节点 ID。
- `display_name`：可选；默认等于 `id`。
- `enabled`：可选；默认 `true`；设为 `false` 时禁止新任务调度到该节点，但不自动迁移或停止已有服务。
- `public_ipv4`：建议填写；用于 DNS 默认值和展示。
- `public_ipv6`：可选；用于自动创建 `AAAA` 记录。
- `token`：必填；该节点的 agent 认证 token；`v1` 先直接写在配置中，后续可替换为公私钥认证。

补充规则：

- `main` 节点必须出现在 `nodes[]` 中。
- `main` 节点 ID 固定为 `main`。
- `composia-meta.yaml` 中 `node` 省略时，默认就是 `main`。
- `agent` 到 `main` 的认证先使用单节点单 token；后续再替换为公私钥模型。
- agent 执行任务时，详细日志流式回传到 `main`，并由 `main.log_dir` 落盘保存。

`main.cli_tokens[]`：

- `name`：必填；token 名称。
- `token`：必填；CLI 访问令牌。
- `enabled`：可选；默认 `true`。
- `comment`：可选；备注用途。

#### `agent` 配置

字段定义：

- `main_addr`：必填；完整 URL；agent 主动连接的 main 地址。
- `node_id`：必填；必须对应 `main.nodes[]` 中的一个节点 ID。
- `token`：必填；与 `main.nodes[]` 中对应节点的 token 匹配。
- `repo_dir`：必填；本地服务包落盘目录。
- `state_dir`：必填；本地少量运行态、任务临时文件、缓冲文件目录。

补充规则：

- `agent` 不需要显式配置监听地址；`v1` 默认由 agent 主动连 main。
- `agent` 不维护节点清单。
- `agent` 不负责解密 secrets，只接收 `main` 下发的运行时文件。

#### CLI 配置

- CLI 使用单独配置文件：`composia-cli.yaml`。
- `v1` 先只支持单 profile。
- 最小字段：`main_addr`、`token_file`。

示例：

```yaml
# main 机器上的 config.yaml
main:
  listen_addr: ":8080"
  public_addr: "https://composia.example.com"
  repo_dir: "/var/lib/composia/repo"
  state_dir: "/var/lib/composia/state"
  log_dir: "/var/log/composia"

  git:
    remote_url: "https://github.com/example/selfhosted.git"
    pull_interval: "1m"
    auth:
      token_file: "/etc/composia/git.token"

  nodes:
    - id: main
      display_name: Main
      enabled: true
      public_ipv4: 203.0.113.10
      public_ipv6: 2001:db8::10
      token: "main-agent-token"

    - id: node-2
      display_name: Tokyo Node
      enabled: true
      public_ipv4: 198.51.100.20
      token: "node-2-token"

  cli_tokens:
    - name: laptop
      token: "cli-token-1"
      enabled: true
      comment: "local admin laptop"
```

```yaml
# agent 机器上的 config.yaml
agent:
  main_addr: "https://composia.example.com"
  node_id: "node-2"
  token: "node-2-token"
  repo_dir: "/var/lib/composia/repo"
  state_dir: "/var/lib/composia/state"
```

```yaml
# CLI 使用的 composia-cli.yaml
main_addr: "https://composia.example.com"
token_file: "/home/alex/.config/composia/token"
```

---

### 5. 服务定义模型（meta.yaml）（v1 正式规范）

固定文件名：`composia-meta.yaml`

设计约束：`meta.yaml` 只描述 `composia` 自己需要的编排信息；资源限制、依赖、hooks、镜像版本等继续交给 `docker compose` 和 `.env` 表达，不在这里重复造一层 DSL。

顶层字段：

- `name`：必填；服务唯一标识；在整个 repo 内必须全局唯一。
- `project_name`：可选；默认等于 `name`。
- `enabled`：可选；默认 `true`。
- `node`：可选；默认 `main`。
- `network`：可选；不写即不接管入口和 DNS。
- `update`：可选；不写即不接管更新。
- `backup`：可选；不写即无备份配置。

校验规则：

- 未知字段直接报错。
- 不允许 `x-*` 自定义扩展字段。
- 所有相对路径都相对于当前服务目录根解析。
- `name` 可以与目录名不同，但不能与 repo 中其他服务重名。

#### `network`

`network.caddy`：

- 可选。
- 支持字段：`enabled`、`source`。
- 如果 `enabled: true`，则 `source` 必填。
- `source` 指向当前服务目录内的 Caddy 片段文件。

`network.dns`：

- 可选。
- `v1` 只支持 `provider: cloudflare`。
- 必填字段：`provider`、`hostname`。
- 可选字段：`record_type`、`value`、`proxied`、`ttl`、`comment`。
- 支持的 `record_type`：`A`、`AAAA`、`CNAME`。
- 如果 `value` 省略，则默认取目标 `node` 的公网地址。
- 如果 `record_type` 省略，则按 `value` 自动判断；若目标 `node` 同时有 IPv4 和 IPv6，则自动创建 `A + AAAA`。

#### `update`

- `v1` 只负责执行更新，不负责发现新 tag 或改写镜像版本。
- 镜像 tag / digest 继续写在 `docker-compose.yaml` 或 `.env` 中。
- 支持字段：`enabled`、`strategy`、`schedule`、`backup_before_update`。
- `strategy` 必填。
- `enabled` 默认 `true`。
- `schedule` 省略表示只允许手动触发。
- `v1` 仅支持 `strategy: pull_and_recreate`。
- 不内建自动回滚、主动探测、业务健康检查。

#### `backup`

- 使用 `jobs` 列表，不再使用固定的 `database/config/files` 顶层键。
- `v1` 的 `jobs[].type` 只支持 `database`、`files`。
- `jobs[].provider` 可省略，默认 `rustic`。
- `jobs[].enabled` 可省略，默认 `true`。
- `jobs[].schedule` 省略表示只允许手动触发。
- `jobs[].retain` 省略表示不自动清理。

`backup.jobs[]` 通用字段：

- `name`：必填；job 名称；在当前服务内唯一。
- `type`：必填；`database` 或 `files`。
- `strategy`：必填。
- `provider`：可选；默认 `rustic`。
- `enabled`：可选；默认 `true`。
- `schedule`：可选；cron 表达式。
- `retain`：可选；保留策略字符串。
- `options`：可选；类型专属参数。

`database` job 规则：

- `v1` 仅支持 `strategy: pgdumpall`。
- `options.service` 必填，值为 `docker-compose.yaml` 内的 Compose service 名。

`files` job 规则：

- `options.include` 必填。
- `include` 允许三种目标：相对路径、绝对路径、Docker volume 名。
- 识别规则：以 `/`、`./`、`../` 开头的视为路径，其他字符串视为 Docker volume 名。

完整示例：

```yaml
name: vaultwarden
project_name: vaultwarden
enabled: true
node: node-2

network:
  caddy:
    enabled: true
    source: ./site_config.caddy
  dns:
    provider: cloudflare
    hostname: vaultwarden.alexma.top
    proxied: true
    ttl: 1
    comment: managed by composia

update:
  enabled: true
  strategy: pull_and_recreate
  schedule: "0 4 * * *"
  backup_before_update: true

backup:
  jobs:
    - name: pg
      type: database
      strategy: pgdumpall
      schedule: "0 2 * * *"
      options:
        service: postgres

    - name: config
      type: files
      strategy: copy
      schedule: "0 3 * * *"
      options:
        include:
          - ./config
          - /srv/vaultwarden/data
          - vaultwarden_data
```

补充约定：

- 服务运行状态以 `docker compose` / 容器状态为准。
- `composia` 不内建主动探测、业务健康检查或自动回滚逻辑。
- 推荐服务目录采用 `.env` + `.secret.env.enc` 方案；运行时由 `main` 解密为 agent 本地的 `.secret.env`，供 `docker compose` 直接使用。

---

### 6. 备份系统（已确定大部分）

- 统一抽象（pre-hook / dump / copy / snapshot / retention）。
- 默认 provider：rustic（内置），其他后续 PR。
- PostgreSQL 在 `v1` 仅支持 `pgdumpall`。
- 备份完成后**必须上报 main** 存入 SQLite。
- 迁移时**强制备份**（即使 meta.yaml 里关闭备份）。
- `v1` 不内建 restore 工作流，默认接受人工恢复。

---

### 7. 服务迁移流程（已确定）

1. 修改 `composia-meta.yaml` 中的 `node`。
2. main 自动备份旧节点。
3. 停止旧节点服务。
4. rsync 数据包到新节点。
5. 新节点启动。
6. 更新 Caddy 配置 + DNS（Cloudflare）。
7. 用户人工验证服务可用性；失败后由用户人工处理。

---

### 8. Secrets 管理

- 服务目录中普通环境变量使用明文 `.env`，机密环境变量使用加密后的 `.secret.env.enc`。
- `docker-compose.yaml` 可直接通过 `env_file` 引用 `.env` 和 `.secret.env`。
- `main` 负责将 `.secret.env.enc` 解密为目标 `agent` 本地的 `.secret.env`。
- `agent` 不承担解密职责，只接收自身需要的运行时 secrets 文件。
- 这样即使脱离 `composia`，用户也可以手动解密 `.secret.env.enc` 后继续使用 `docker compose` 运维。

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

- 在线编辑仓库文件（`docker-compose.yaml`、`.env`、`composia-meta.yaml`、Caddy 片段）。
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
- 任务日志：详细执行日志单独落在 `main.log_dir` 文件中，SQLite 不承载大段日志内容
