### 1. 项目目标（一句话定位）

**Composia = 服务声明 + 部署控制面 + 运行节点执行面**

核心目标：做一个**以服务为核心**的自建服务管理器，支持单节点/多节点，重点解决实际自建运维中的备份、迁移、更新、监控等痛点，而不是做一个普通的 Docker Compose GUI。

### 2. 核心设计原则（已全部确定）

- **以服务为主**：所有操作（部署、备份、迁移、监控）都围绕“服务”（如 stalwart、vaultwarden）而不是文件类型。
- **一个服务只部署在一个节点**：v1 不支持多副本/高可用（后续 v2 再单独设计）。
- **单主控制面**：永远只有 1 个 active controller + N 个 agent，不做 active-active。
- **同一二进制不同角色**：`composia` 一个程序，通过启动参数切换 `controller` / `agent` 模式。
- **controller 可选同机运行 agent**：`controller` 所在机器可以额外运行一个本地 `agent` 进程；当需要把服务部署到该机器时，推荐使用保留节点 ID `main`；所有服务任务都统一经 agent 执行，`controller` 不内嵌特殊本地执行器。
- **Git 只存声明**：Git 负责服务定义、meta.yaml、模板、历史；运行时状态全部进 SQLite。
- **controller 持全量，agent 只持本地**：agent 只保存自己需要运行的服务包，减少暴露和同步量。
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

- **Controller**（控制面）：Git 拉取、配置渲染、任务调度、备份编排、Caddy 片段分发、状态数据库、API、Web UI、CLI 入口；如需在本机承载服务，可额外运行一个本地 agent。
- **Agent**（执行面）：接收 controller 下发的服务包、本地落盘、执行 docker compose up/down/pull、执行备份/迁移步骤、上报状态；如果配置了本地 `main` 节点，其执行路径与普通节点完全一致。
- **通信方式**：ConnectRPC + 预共享 token 认证

---

### 4. 目录结构（推荐结构，已基本确定）

```text
composia/                  # 程序安装目录（controller 和 agent 都一样）
  config.yaml
  state/composia.db
  logs/
    tasks/

repo/                      # Git 仓库根目录（controller 全量，agent 只本地服务）
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
      site-generated/        # agent 本地生成，非 Git 真相源
```

agent 本地只会保留自己需要运行的服务 + caddy。
agent 落盘后，服务目录中会有解密后的 `.secret.env` 供 `docker compose` 使用，该文件不进 Git。
当 `controller` 与本地 agent 部署在同一台机器时，`controller.repo_dir` 必须与 `agent.repo_dir` 分开，前者是全量 Git 工作树，后者是本地服务 bundle 落盘目录。

---

### 4.1 运行配置（config.yaml）（v1 正式规范）

设计约束：

- `config.yaml` 是每台机器本地独立维护的配置文件，不共享同一份物理文件。
- 启动角色由命令决定：`composia controller` / `composia agent`。
- 配置结构按角色分区：`controller:` 和 `agent:`。
- 实际部署时，控制面机器至少写 `controller:`；若还要在本机承载服务，再额外写 `agent:` 并以第二个进程运行；普通 `agent` 主机通常只写 `agent:`。

#### `controller` 配置

字段定义：

- `listen_addr`：必填；本机监听地址；用于绑定 API / Web 服务。
- `controller_addr`：必填；完整 URL；供 agent 和 CLI 连接 controller；不要求必须是公网地址。
- `repo_dir`：必填；本地 Git 工作目录。
- `state_dir`：必填；SQLite 和其他本地状态文件目录。
- `log_dir`：必填；任务详细日志目录。
- `git`：可选；不写时表示使用默认本地 Git 行为；`controller.git` 只用于覆盖默认作者信息或配置远程跟踪参数。
- `nodes`：必填；节点清单；凡是会运行 `agent` 的节点都必须显式列在这里；仅部署控制面时可以不包含 `main`。
- `cli_tokens`：可选；CLI 访问 controller 的令牌列表；`v1` 先手写在配置中。
- `dns`：可选；全局 DNS provider 配置；服务侧声明引用这里的 provider 凭据。
- `backup`：可选；全局备份 provider 配置；服务侧 `backup.data[]` 引用这里的 provider 实例。
- `secrets`：可选；全局 secrets 解密配置；repo 中存在 `.secret.env.enc` 时需要。

`controller.git`：

- `controller.git` 没有单独的 enable 开关；`repo_dir` 始终按 Git 工作树处理。
- 如果 `controller.git` 整段省略，则使用默认本地 Git 模式。
- 如果 `remote_url` 不存在，则视为本地 Git 模式：允许 Web/API/系统写 repo 并生成本地 commit，但不做 auto pull / push。
- 如果 `remote_url` 存在，则视为远程跟踪模式：允许 Web/API/系统写 repo，并在 commit 后 push；同时支持 auto pull。
- `remote_url`：可选；存在时启用托管拉取。
- `branch`：可选；默认 remote HEAD。
- `pull_interval`：在托管拉取模式下必填；使用 Go duration 格式，如 `30s`、`5m`。
- `auth.token_file`：可选；Git 认证 token 文件路径；为未来扩展 SSH/key 留结构空间。
- `author_name`：可选；默认 `Composia`；用于生成 commit。
- `author_email`：可选；默认 `composia@localhost`；用于生成 commit。

`controller.nodes[]`：

- `id`：必填；节点 ID。
- `display_name`：可选；默认等于 `id`。
- `enabled`：可选；默认 `true`；设为 `false` 时禁止新任务调度到该节点，但不自动迁移或停止已有服务。
- `public_ipv4`：建议填写；用于 DNS 默认值和展示。
- `public_ipv6`：可选；用于自动创建 `AAAA` 记录。
- `token`：必填；该节点的 agent 认证 token；`v1` 先直接写在配置中，后续可替换为公私钥认证。

补充规则：

- 只有当 `controller` 主机也运行本地 agent 时，`main` 节点才需要出现在 `nodes[]` 中。
- 本地 `main` 节点的保留 ID 为 `main`。
- `composia-meta.yaml` 中 `node` 省略时，默认目标节点是 `main`；如果当前未配置 `id: main` 的节点，则校验报错。
- `controller` 主机上的本地 `agent.node_id` 必须等于 `main`。
- 当 `controller` 与本地 agent 部署在同一台机器时，`controller.repo_dir` 与 `agent.repo_dir` 不得复用同一路径。
- `controller.repo_dir` 必须是一个可写的 Git working tree；`v1` 不支持完全脱离 Git 的 repo 目录。
- `agent` 到 `controller` 的认证先使用单节点单 token；后续再替换为公私钥模型。
- agent 执行任务时，详细日志流式回传到 `controller`，并由 `controller.log_dir` 落盘保存。
- 在 Git 模式下，所有 repo 读写、auto pull、bundle 打包都必须经过同一个 repo lock 串行化。
- Web/API/系统对 repo 的任何写操作都必须遵循：若配置了远程则先 `fetch + fast-forward`，再写文件、校验、`commit`；若配置了远程则继续 `push`。
- 没有远程仓库时，新 revision 在本地 commit 成功后立即生效；有远程仓库时，本地 commit 成功后即作为 controller 当前有效 revision，但 repo 可能暂时处于“远程未同步”状态。
- 若 `push` 成功，则该 revision 同步到远程；若 `push` 失败，则不回滚本地 `HEAD`，但 controller 必须显式记录并暴露当前 repo sync 状态，便于 Web UI / CLI / 后续任务感知“本地已提交、远程未同步”。
- `repo_dir` 在空闲状态下必须保持 clean working tree；`push` 失败时也不得留下脏工作区。

`controller.cli_tokens[]`：

- `name`：必填；token 名称。
- `token`：必填；CLI 访问令牌。
- `enabled`：可选；默认 `true`。
- `comment`：可选；备注用途。

`controller.dns.cloudflare`：

- 当任一服务使用 `network.dns.provider: cloudflare` 时必填。
- `api_token_file`：必填；Cloudflare API token 文件路径。

`controller.backup.rustic`：

- 当任一备份数据项使用 `provider: rustic` 时必填。
- `repository`：必填；rustic repository 地址。
- `password_file`：必填；rustic repository 密码文件。
- `env_files`：可选；provider 额外环境变量文件列表，例如 S3 凭据。

`controller.secrets`：

- `v1` 固定使用 `provider: age`。
- `identity_file`：必填；`controller` 机器用于解密 `.secret.env.enc` 的 age 私钥文件。
- `recipient_file`：必填；重新加密 `.secret.env.enc` 时使用的 age recipient 列表文件。
- `armor`：可选；默认 `true`。

#### `agent` 配置

字段定义：

- `controller_addr`：必填；完整 URL；agent 主动连接的 controller 地址。
- `node_id`：必填；必须对应 `controller.nodes[]` 中的一个节点 ID。
- `token`：必填；与 `controller.nodes[]` 中对应节点的 token 匹配。
- `repo_dir`：必填；本地服务包落盘目录。
- `state_dir`：必填；本地少量运行态、任务临时文件、缓冲文件目录。
- `caddy.generated_dir`：可选；agent 本地给 Caddy 使用的生成目录；默认 `repo_dir/caddy/config/site-generated`。

补充规则：

- `agent` 不需要显式配置监听地址；`v1` 默认由 agent 主动连 controller。
- `agent` 不维护节点清单。
- `agent` 不负责解密 secrets，只接收 `controller` 下发的运行时文件。
- 本地 agent 与远端 agent 无语义差别，只是与 `controller` 部署在同一台机器。
- `caddy.generated_dir` 只允许在 agent 本地统一配置，不允许服务在 `meta.yaml` 中自定义目标目录。

#### CLI 配置

- CLI 使用单独配置文件：`composia-cli.yaml`。
- `v1` 先只支持单 profile。
- 最小字段：`controller_addr`、`token_file`。

示例：

```yaml
# controller 机器上的 config.yaml
controller:
  listen_addr: ":8080"
  controller_addr: "https://composia.example.com"
  repo_dir: "/var/lib/composia/repo-controller"
  state_dir: "/var/lib/composia/state-controller"
  log_dir: "/var/log/composia"

  git:
    remote_url: "https://github.com/example/selfhosted.git"
    branch: "main"
    pull_interval: "1m"
    author_name: "Composia"
    author_email: "composia@example.com"
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

  dns:
    cloudflare:
      api_token_file: "/etc/composia/cloudflare.token"

  backup:
    rustic:
      repository: "s3:https://s3.example.com/composia"
      password_file: "/etc/composia/rustic.password"
      env_files:
        - "/etc/composia/rustic.env"

  secrets:
    provider: age
    identity_file: "/etc/composia/age.key"
    recipient_file: "/etc/composia/age.recipients"
    armor: true

agent:
  controller_addr: "https://composia.example.com"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/var/lib/composia/repo-agent"
  state_dir: "/var/lib/composia/state-agent"
```

```yaml
# agent 机器上的 config.yaml
agent:
  controller_addr: "https://composia.example.com"
  node_id: "node-2"
  token: "node-2-token"
  repo_dir: "/var/lib/composia/repo"
  state_dir: "/var/lib/composia/state"
```

```yaml
# CLI 使用的 composia-cli.yaml
controller_addr: "https://composia.example.com"
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
- `node`：可选；若省略则默认 `main`，但要求当前已配置 `id: main` 的节点，否则校验报错。
- `network`：可选；不写即不接管入口和 DNS。
- `update`：可选；不写即不接管更新。
- `data_protect`：可选；不写即无受管数据声明。
- `backup`：可选；不写即无日常备份配置。
- `migrate`：可选；不写即迁移时不额外搬运受管数据。

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
- `source` 指向当前服务目录内的 Caddy 片段文件，允许每个服务自行选择相对路径。
- agent 会把 `source` 对应文件复制到本节点的 `caddy.generated_dir` 中，文件名固定为 `<service-name>.caddy`。
- `caddy.generated_dir` 中的文件是节点本地派生物，不进入 Git，不允许手动作为真相源长期维护。
- 每次相关变更都对目标节点执行“全量重建 `caddy.generated_dir` -> `caddy validate` -> `caddy reload`”流程，而不是增量修补单个文件。
- 如果校验或 reload 失败，则保留旧的生成目录和旧配置，不让半成品映射生效。

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

#### `data_protect`

- 使用 `data` 列表声明服务内可独立导出/恢复的数据保护单元。
- 这里的 `data` 是 `composia` 视角下的“可执行保护单元”，不强求与业务语义中的唯一数据实体一一对应；同一份底层业务数据允许定义多个 `data` 项，例如 `pg-logical` 与 `pg-volume`。
- `v1` 不再把数据库/文件拆成固定顶层键，而是统一放在 `data_protect.data[]` 中。

`data_protect.data[]` 通用字段：

- `name`：必填；数据项名称；在当前服务内唯一。
- `backup`：可选；该数据项的导出定义。
- `restore`：可选；该数据项的恢复定义。
- 被 `backup.data[]` 引用的数据项必须定义 `backup`。
- 被 `migrate.data[]` 引用的数据项必须同时定义 `backup` 和 `restore`。

`data_protect.data[].backup` / `data_protect.data[].restore`：

- 都使用统一的 `strategy` + 策略专属字段结构。
- `v1` 不再额外引入 `type` 字段；字段集合由 `strategy` 语义决定。
- `database.*` 策略使用数据库专属字段；`files.*` 策略使用文件/目录/volume 专属字段。

`database.*` 规则：

- `backup.strategy` 在 `v1` 仅支持 `database.pgdumpall`。
- `restore.strategy` 在 `v1` 仅支持 `database.pgimport`。
- `service` 必填，值为 `docker-compose.yaml` 内的 Compose service 名。

`files.*` 规则：

- `include` 必填。
- `include` 允许三种目标：相对路径、绝对路径、Docker volume 名。
- 识别规则：以 `/`、`./`、`../` 开头的视为路径，其他字符串视为 Docker volume 名。
- `files.tar_after_stop` 这类策略名显式表示其需要在源服务停止后执行；`v1` 约定策略名本身应体现这类前置条件。

#### `backup`

- 使用 `data` 列表引用需要纳入日常备份的数据项。
- `data[].provider` 可省略，默认 `rustic`。
- `data[].enabled` 可省略，默认 `true`。
- `data[].schedule` 省略表示只允许手动触发。
- `data[].retain` 省略表示不自动清理。

`backup.data[]` 字段：

- `name`：必填；引用的 `data_protect.data[].name`。
- `provider`：可选；默认 `rustic`。
- `enabled`：可选；默认 `true`。
- `schedule`：可选；cron 表达式。
- `retain`：可选；保留策略字符串。

#### `migrate`

- 使用 `data` 列表引用迁移时需要导出、传输、恢复的数据项。
- `migrate` 不复用 `backup.data[]` 的 `schedule`、`retain`、`provider` 语义；它只决定迁移时选哪些数据项。
- 如果一个服务没有 `migrate.data[]`，则迁移只搬运服务声明、运行时文件和入口切换，不保证带走额外业务数据。

`migrate.data[]` 字段：

- `name`：必填；引用的 `data_protect.data[].name`。
- `enabled`：可选；默认 `true`。

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

data_protect:
  data:
    - name: pg-logical
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres

    - name: pg-volume
      backup:
        strategy: files.tar_after_stop
        include:
          - postgres_data
      restore:
        strategy: files.untar
        include:
          - postgres_data

    - name: config
      backup:
        strategy: files.copy
        include:
          - ./config
          - /srv/vaultwarden/data
          - vaultwarden_data
      restore:
        strategy: files.copy
        include:
          - ./config
          - /srv/vaultwarden/data
          - vaultwarden_data

backup:
  data:
    - name: pg-logical
      schedule: "0 2 * * *"
      retain: "7d"

    - name: config
      schedule: "0 3 * * *"

migrate:
  data:
    - name: pg-volume
    - name: config
```

补充约定：

- 服务运行状态以 `docker compose` / 容器状态为准。
- `composia` 不内建主动探测、业务健康检查或自动回滚逻辑。
- 推荐服务目录采用 `.env` + `.secret.env.enc` 方案；运行时由 `controller` 解密为 agent 本地的 `.secret.env`，供 `docker compose` 直接使用。

---

### 6. 备份系统（已确定大部分）

- 统一抽象（pre-hook / dump / copy / snapshot / retention）。
- 默认 provider：rustic（内置），其他后续 PR。
- `data_protect.data[]` 是服务数据保护的声明真相源；`backup` 和 `migrate` 都只负责选择引用哪些数据项。
- PostgreSQL 在 `v1` 仅支持 `database.pgdumpall` 导出和 `database.pgimport` 恢复。
- 备份完成后**必须上报 controller** 存入 SQLite。
- 常规 `backup` 任务执行的是 `backup.data[]` 引用的数据项，并使用对应 `data_protect.data[].backup` 定义导出数据。
- `v1` 不提供独立的 restore API / 页面，默认接受人工恢复；但迁移任务内部允许消费迁移所选数据项的产物完成恢复/导入。
- 迁移使用 `migrate.data[]` 选择数据项，并直接复用对应 `data_protect.data[].backup` 和 `restore` 定义；如果想让迁移和日常备份使用不同格式，应定义不同的数据项，例如 `pg-logical` 与 `pg-volume`。
- 迁移产物介质在 `v1` 固定为本地临时落盘：源节点先把导出产物写入本地临时目录，再传输到目标节点本地临时目录供恢复使用；不使用对象存储、controller 中转仓库或 agent 间持久共享存储。
- 需要随迁移带走的数据库、目录、绝对路径、Docker volume 必须在 `migrate.data[]` 中显式声明，否则 `composia` 不保证迁移完整性。

---

### 7. 服务迁移流程（v1 正式语义）

- 迁移入口是 `MigrateService(target_node_id)`，用户不需要预先手改 `composia-meta.yaml`。
- 迁移任务创建时，`controller` 会锁定当前已提交的 `repo_revision`、当前 `source_node_id` 和请求中的 `target_node_id`。
- 在迁移成功并完成流量切换前，Git 中的 `composia-meta.yaml.node` 仍保持旧值。
- 只有目标节点拉起成功且后续步骤完成后，`controller` 才会尝试把 `node` 字段改为目标节点并执行 `commit`；若配置了远程仓库则继续 `push`。
- 迁移完成运行态切换后，在人工验证和 repo 对账完成前，`migrate` 任务进入 `awaiting_confirmation`；该服务在此期间不得继续创建其他任务。
- `v1` 不提供迁移失败后的自动回滚、自动清理或自动恢复源节点工作流，统一由用户人工处理。
- Caddy 片段的真相源始终是服务目录中的 `network.caddy.source`；节点上的 `caddy.generated_dir` 只是运行时复制产物。

执行步骤：

1. 校验目标节点存在、已启用、在线，且服务当前没有冲突任务。
2. 基于当前提交的 repo revision 创建单个 `migrate` 任务，并把 `source_node_id` / `target_node_id` 写入任务参数。
3. 解析该服务全部启用的 `migrate.data[]`，校验引用的 `data_protect.data[]` 全部存在，且每项都同时定义 `backup` 和 `restore`。
4. 在源节点按迁移所选数据项的 `backup` 定义执行导出；若某个策略要求先停机，则必须先停止源节点服务再执行该数据项导出。
5. 若源节点服务尚未停止，则在目标节点恢复和启动前停止源节点服务，保证服务不会同时运行在两个节点。
6. 将迁移产物和所需运行时文件传输到目标节点。
7. 在目标节点按对应数据项的 `restore` 定义恢复数据，并使用同一份服务 bundle 启动服务。
8. 刷新目标节点的 Caddy 生成目录并 reload，同时在源节点全量重建其 Caddy 生成目录以移除旧服务片段并 reload；然后执行 DNS 更新。
9. 完成上述执行步骤后，`controller` 尝试修改 `composia-meta.yaml` 中该服务的 `node`，生成 commit；若配置了远程仓库则继续 push 到跟踪分支。
10. 用户人工验证业务可用性并完成 repo 对账；如果第 9 步之前失败，则 Git desired state 保持源节点且任务标记为 `failed`；如果运行态已切换但 repo 写回 / push / 人工对账尚未完成，则任务进入 `awaiting_confirmation`，并提示“运行态已迁移但 repo 仍需人工对账”。

---

### 8. Secrets 管理

- 候选方案：
- 方案 A：纯明文 `.env`
- 优点：最简单，完全兼容 `docker compose`。
- 缺点：不能进 Git，不满足本项目的声明式仓库目标。
- 方案 B：`.secret.env.enc` + `age`
- 优点：文件格式简单、单二进制工具链、适合存 Git、用户脱离 `composia` 仍可手动解密继续运维。
- 缺点：需要 `controller` 持有解密私钥；Web/CLI 编辑 secrets 时需要服务端代为重新加密。
- 方案 C：`sops`
- 优点：生态成熟，支持多个后端和更复杂的密钥管理。
- 缺点：引入额外概念和依赖，对 v1 偏重。
- 方案 D：外部 secret manager（Vault / 1Password / 云厂商）
- 优点：能力最强。
- 缺点：明显超出 v1 范围，也违背“自建服务管理器 keep it simple”的方向。

- `v1` 正式选型：方案 B，使用 `age` 加密的 `.secret.env.enc`。
- 服务目录中普通环境变量使用明文 `.env`，机密环境变量使用加密后的 `.secret.env.enc`。
- `.secret.env.enc` 对应的明文内容固定为 dotenv 文本；运行时解密结果文件名为 `.secret.env`。
- `docker-compose.yaml` 可直接通过 `env_file` 引用 `.env` 和 `.secret.env`。
- `controller` 负责将 `.secret.env.enc` 解密为目标 `agent` 本地的 `.secret.env`。
- `agent` 不承担解密职责，只接收自身需要的运行时 secrets 文件。
- Git 工作树中只保存 `.secret.env.enc`，不保存 `.secret.env` 明文。
- `controller` 在打包 bundle 或处理 secrets 编辑时，可以在进程内存或临时文件中持有明文，但任务完成后必须清理，不得把 `.secret.env` 留在 `controller.repo_dir`。
- 这样即使脱离 `composia`，用户也可以手动解密 `.secret.env.enc` 后继续使用 `docker compose` 运维。

`controller.secrets`：

- `provider`：`v1` 固定为 `age`。
- `identity_file`：必填；`controller` 用于解密的 age 私钥文件。
- `recipient_file`：必填；重新加密 `.secret.env.enc` 时使用的 age recipient 列表文件。
- `armor`：可选；默认 `true`；控制生成的 `.secret.env.enc` 是否使用 ASCII armored 格式。

补充规则：

- `recipient_file` 至少包含一个 recipient；`v1` 允许多个 recipient，便于用户自己保留额外解密能力。
- Web UI / CLI 不默认直接编辑 `.secret.env.enc` 密文本身，而是通过 secrets 专用 API 读取明文并由服务端重新加密写回。
- secrets 写回 repo 的语义与普通 repo 文件一致：生成 Git commit；若配置了远程仓库则继续 push。
- `v1` 不做密钥轮换自动化；更换 recipient 时由用户手动触发“重新加密全部 secrets”。
- `v1` 不支持跨服务共享 secret 引用、模板插值或外部 secret provider。

---

### 9. AI 助手 / MCP / Skills

`v1` 不接模型能力，只保留文档入口或操作说明页。

---

### 10. 节点注册与 Controller 高可用

- `v1` 采用完全手动注册。
- `controller` 手动维护节点列表。
- 如果需要把服务部署到 `controller` 主机，则该机器还需以 `node_id: main` 运行一个本地 agent。
- `agent` 在 `config.yaml` 中手动配置 `controller` 地址、节点 ID、预共享 token。
- 不做 `controller` 高可用；`controller` 挂了就接受人工切换和人工恢复。

---

### 10.1 任务模型（v1 正式规范）

设计目标：

- API / Web / CLI 只负责创建任务，不同步等待长时间执行结束。
- `controller` 使用持久任务队列，任务先写入 SQLite，再由后台 worker 执行。
- `v1` 队列只追求稳定和可观测性，不追求复杂调度能力。

队列与并发：

- `v1` 使用 `controller` 内部持久任务队列。
- 创建任务成功后应立即返回 `task_id`。
- `v1` 采用全局串行执行模型：同一时刻只执行一个任务；处于 `awaiting_confirmation` 的任务不占执行槽位。
- 如果同一服务已经有 `pending`、`running` 或 `awaiting_confirmation` 任务，新任务直接拒绝。
- 如果同一服务已经有 `pending`、`running` 或 `awaiting_confirmation` 任务，所有触及该服务目录的 repo 写操作也必须直接拒绝；这里的“触及”包括该服务目录下的声明文件、compose 文件、普通配置文件和 secrets 文件。
- 这种限制只作用于正式写入 repo 的操作；未提交草稿只能保留在客户端或独立 draft store 中，不能写入 `controller.repo_dir` 形成 dirty working tree。
- 不做离线补偿队列；目标 `agent` 离线时，任务执行直接失败。

任务实例：

- 每次触发都创建新的 `task_id`。
- 不提供传统意义上的 `RetryTask`；如需再次执行，统一创建新的任务实例。
- 任务列表默认按时间倒序展示。

支持的任务类型：

- `deploy`
- `stop`
- `restart`
- `update`
- `backup`
- `migrate`
- `dns_update`
- `caddy_reload`
- `prune`

任务语义：

- `deploy`：按当前 repo 内容渲染并执行 `docker compose up -d`。
- `update`：先执行 `docker compose pull`，再继续 `deploy` 流程。
- `backup`：默认执行该服务全部启用的 `backup.data[]`；也支持通过 `data_names` 指定单个或多个数据项。
- `migrate`：是单个任务，内部按步骤推进，而不是拆成多个独立任务；任务参数必须记录 `source_node_id` 和 `target_node_id`，运行态切换完成后由 `controller` 在 `persist_repo` 阶段尝试把新的 `node` 写回 repo 并生成 Git commit；若配置了远程仓库则继续 push；在人工验证和 repo 对账完成前，任务进入 `awaiting_confirmation`。
- `dns_update`：按服务当前声明刷新 DNS 记录。
- `caddy_reload`：对指定节点执行一次 Caddy reload。
- `prune`：对指定节点执行容器 / 镜像 / volume 清理；具体清理范围由参数控制。
- 所有任务在创建时都绑定一个确定的 `repo_revision`；agent 只执行该 revision 对应的 bundle，避免执行过程中受到后续 Git 变更影响。
- `migrate` 在进入 `persist_repo` 前必须重新获取 repo lock 并检查最新 `HEAD`；如果 `HEAD` 仅包含与该服务无关的变更，则允许基于最新 `HEAD` 补写该服务的最终 `node` 并生成新的 commit；如果最新变更触及该服务目录、导致服务声明失效，或触及迁移依赖的相关全局配置，则不得自动写回 repo，而是保留“运行态已迁移但 repo 待人工对账”的状态。

任务状态：

- `pending`
- `running`
- `awaiting_confirmation`
- `succeeded`
- `failed`
- `cancelled`

状态规则：

- `pending` 表示任务已入队，尚未开始执行。
- `running` 表示任务已开始执行，且当前仍占用 worker 执行自动步骤。
- `awaiting_confirmation` 表示自动执行步骤已结束，worker 已释放，但仍在等待人工验证或 repo 对账。
- `succeeded` / `failed` / `cancelled` 为终态。
- 对 `migrate` 而言，自动执行步骤完成后会从 `running` 转入 `awaiting_confirmation`，而不是继续停留在 `running`。
- `v1` 不支持手动取消接口，因此 `cancelled` 主要用于定时任务因冲突被跳过等场景。

任务来源：

- `source` 必须记录。
- `v1` 先支持：`web`、`cli`、`schedule`、`system`。
- `triggered_by` 只记录来源名，不记录更细粒度调用者身份。

任务关联字段：

- `task_id`
- `type`
- `source`
- `triggered_by`
- `service_name`
- `node_id`
- `status`
- `created_at`
- `started_at`
- `finished_at`
- `repo_revision`
- `result_revision`
- `error_summary`

步骤模型：

- 一个用户动作对应一个任务，任务内部只记录步骤摘要。
- 每个步骤使用固定枚举名，便于前后端和日志统一。
- 每个步骤至少记录：步骤名、状态、开始时间、结束时间。
- 步骤日志仍写入任务日志文件，不写入 SQLite 大字段。

建议的步骤枚举：

- `render`
- `pull`
- `backup`
- `compose_down`
- `compose_up`
- `transfer`
- `restore`
- `dns_update`
- `caddy_reload`
- `prune`
- `persist_repo`
- `finalize`

失败与重试：

- `v1` 不做自动重试。
- 失败后仅支持手动 `RunAgain`。
- `RunAgain` 会创建新的任务实例，而不是复用原 `task_id`。
- `RunAgain` 固定基于当前最新 `HEAD` 和当前有效配置重新创建任务，不复用原任务的 `repo_revision`。
- `migrate` 任务只要任一步骤失败，整个任务结果即为 `failed`。
- `backup` 指定了不存在的 `data_names` 时，任务直接 `failed`。
- `migrate` 失败后不做自动回滚、自动清理或自动恢复源节点；`v1` 默认由用户根据任务日志和备份产物人工处理。

超时与恢复：

- `v1` 支持一个全局默认任务超时。
- 超时后任务标记为 `failed`。
- 如果 `controller` 在任务执行中重启，原来的 `running` 任务在恢复阶段统一标记为 `failed`；`awaiting_confirmation` 任务保持原状态。
- `system` 来源在 `v1` 先只作为保留枚举，不强依赖具体自动触发场景。

日志：

- 任务详细日志统一流式回传到 `controller`。
- 详细日志按 `task_id` 落在 `controller.log_dir` 中。
- SQLite 只保存结构化状态、步骤摘要和错误摘要，不保存大段执行输出。

定时任务冲突：

- 定时触发的任务如果遇到服务冲突，不排队等待。
- 此类任务标记为 `cancelled`，用于表示“已触发但未执行”。

---

### 10.2 SQLite 模型（v1 正式规范）

设计约束：

- SQLite 只保存结构化状态和历史记录，不保存 Git 声明真相源。
- 节点配置真相源仍然是 `config.yaml` 中的 `controller.nodes[]`，数据库中的节点信息只作为运行态快照。
- 详细任务日志仍然落在 `controller.log_dir` 文件中，不写入 SQLite。
- `v1` 历史默认永久保留，不做自动清理。
- 主键默认使用字符串 ID。
- 时间字段统一使用 UTC RFC3339 文本。
- 枚举值由代码常量和文档约定维护；数据库直接存文本值，必要时可加 `CHECK` 约束。
- 能加外键的地方尽量加外键。
- `services` 和 `nodes` 作为“历史注册表 + 当前快照”，不会因为 Git / config 中被删除就物理删行，而是通过 presence 标记表示当前是否仍被声明。

#### 核心表

`nodes`

用途：保存节点运行态快照和历史注册状态，而不是节点配置真相源。

建议字段：

- `node_id` TEXT PRIMARY KEY
- `is_configured` INTEGER NOT NULL
- `is_online` INTEGER NOT NULL
- `last_heartbeat` TEXT
- `agent_version` TEXT
- `docker_server_version` TEXT
- `disk_total_bytes` INTEGER
- `disk_free_bytes` INTEGER

说明：

- `node_id` 必须对应 `controller.nodes[]` 中的节点 ID。
- `is_configured` 表示该节点当前是否仍存在于 `controller.nodes[]`。
- `is_online` 由 `controller` 根据最近心跳计算并刷新。
- 节点公网 IP、token、display_name、enabled 等配置仍以 `config.yaml` 为准，不要求冗余写入 SQLite。
- `docker_server_version`、`disk_total_bytes`、`disk_free_bytes` 来自最近一次心跳摘要，用于节点详情页展示。

`services`

用途：保存服务的最小运行态摘要和历史注册状态，不复制完整 repo 配置。

建议字段：

- `service_name` TEXT PRIMARY KEY
- `is_declared` INTEGER NOT NULL
- `runtime_status` TEXT NOT NULL
- `last_task_id` TEXT
- `updated_at` TEXT NOT NULL

外键：

- `last_task_id` -> `tasks.task_id`

说明：

- `service_name` 对应 `composia-meta.yaml` 中的 `name`。
- `is_declared` 表示该服务当前是否仍存在于 Git repo 的最新已解析声明中。
- `runtime_status` 在 `v1` 固定为：`running`、`stopped`、`error`、`unknown`。
- `services` 表在 Git 解析完成后刷新服务存在性，在任务结束后刷新运行态和最近任务；服务从 repo 删除时不删历史行，只将 `is_declared` 置为 `0`。

`tasks`

用途：保存任务实例和执行结果，是任务队列和历史列表的主表。

建议字段：

- `task_id` TEXT PRIMARY KEY
- `type` TEXT NOT NULL
- `source` TEXT NOT NULL
- `triggered_by` TEXT
- `service_name` TEXT
- `node_id` TEXT
- `status` TEXT NOT NULL
- `params_json` TEXT
- `log_path` TEXT
- `repo_revision` TEXT
- `result_revision` TEXT
- `attempt_of_task_id` TEXT
- `created_at` TEXT NOT NULL
- `started_at` TEXT
- `finished_at` TEXT
- `error_summary` TEXT

外键：

- `service_name` -> `services.service_name`
- `node_id` -> `nodes.node_id`
- `attempt_of_task_id` -> `tasks.task_id`

说明：

- `task_id` 每次触发都新建，不复用。
- `type` 在 `v1` 先支持：`deploy`、`stop`、`restart`、`update`、`backup`、`migrate`、`dns_update`、`caddy_reload`、`prune`。
- `status` 在 `v1` 固定为：`pending`、`running`、`awaiting_confirmation`、`succeeded`、`failed`、`cancelled`。
- `source` 在 `v1` 先支持：`web`、`cli`、`schedule`、`system`。
- `triggered_by` 只记录来源名。
- `params_json` 用于保存调用参数快照，例如 `backup.data_names`。
- `log_path` 记录该任务在 `controller.log_dir` 下对应的日志文件路径。
- `repo_revision` 表示任务创建时绑定的输入 Git revision。
- `result_revision` 用于记录任务过程中写 repo 后产生的新 revision，例如迁移成功后写回新的 `node`。
- `attempt_of_task_id` 用于关联“重试来源任务”。

`task_steps`

用途：保存任务内部步骤摘要。

建议字段：

- `task_id` TEXT NOT NULL
- `step_name` TEXT NOT NULL
- `status` TEXT NOT NULL
- `started_at` TEXT
- `finished_at` TEXT

主键与外键：

- PRIMARY KEY (`task_id`, `step_name`)
- `task_id` -> `tasks.task_id`

说明：

- 一个任务内部只记录步骤摘要，不单独落步骤日志。
- `step_name` 在 `v1` 使用固定枚举，例如：`render`、`pull`、`backup`、`compose_down`、`compose_up`、`transfer`、`restore`、`dns_update`、`caddy_reload`、`prune`、`persist_repo`、`finalize`。
- `status` 与 `tasks.status` 基本一致，但步骤级状态通常只使用：`pending`、`running`、`succeeded`、`failed`、`cancelled`；`awaiting_confirmation` 仅用于任务级状态。

`backups`

用途：保存每个备份数据项的结果产物记录，而不是只依附在任务日志里。

建议字段：

- `backup_id` TEXT PRIMARY KEY
- `task_id` TEXT NOT NULL
- `service_name` TEXT NOT NULL
- `data_name` TEXT NOT NULL
- `status` TEXT NOT NULL
- `started_at` TEXT NOT NULL
- `finished_at` TEXT
- `artifact_ref` TEXT
- `error_summary` TEXT

外键：

- `task_id` -> `tasks.task_id`
- `service_name` -> `services.service_name`

说明：

- 一个 `backup` 任务可以生成多条 `backups` 记录，每个数据项一条。
- `status` 与 `tasks.status` 使用同一套枚举。
- 无论成功还是失败，每个备份数据项都应落一条记录。
- `artifact_ref` 用于保存 provider 返回的产物引用，例如 rustic snapshot ID。

#### 刷新时机

- `nodes`：agent 心跳上报后刷新；配置重载时刷新 `is_configured`。
- `services`：Git 解析完成后刷新服务存在性；任务结束后刷新 `runtime_status`、`last_task_id`、`updated_at`。
- `tasks`：任务创建、开始、结束时更新。
- `task_steps`：步骤开始和结束时更新。
- `backups`：备份数据项结束时写入或更新。

#### 最小 DDL 草案

```sql
CREATE TABLE nodes (
  node_id TEXT PRIMARY KEY,
  is_configured INTEGER NOT NULL,
  is_online INTEGER NOT NULL,
  last_heartbeat TEXT,
  agent_version TEXT,
  docker_server_version TEXT,
  disk_total_bytes INTEGER,
  disk_free_bytes INTEGER
);

CREATE TABLE services (
  service_name TEXT PRIMARY KEY,
  is_declared INTEGER NOT NULL,
  runtime_status TEXT NOT NULL,
  last_task_id TEXT,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (last_task_id) REFERENCES tasks(task_id)
);

CREATE TABLE tasks (
  task_id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  source TEXT NOT NULL,
  triggered_by TEXT,
  service_name TEXT,
  node_id TEXT,
  status TEXT NOT NULL,
  params_json TEXT,
  log_path TEXT,
  repo_revision TEXT,
  result_revision TEXT,
  attempt_of_task_id TEXT,
  created_at TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  error_summary TEXT,
  FOREIGN KEY (service_name) REFERENCES services(service_name),
  FOREIGN KEY (node_id) REFERENCES nodes(node_id),
  FOREIGN KEY (attempt_of_task_id) REFERENCES tasks(task_id)
);

CREATE TABLE task_steps (
  task_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT,
  finished_at TEXT,
  PRIMARY KEY (task_id, step_name),
  FOREIGN KEY (task_id) REFERENCES tasks(task_id)
);

CREATE TABLE backups (
  backup_id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  service_name TEXT NOT NULL,
  data_name TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  artifact_ref TEXT,
  error_summary TEXT,
  FOREIGN KEY (task_id) REFERENCES tasks(task_id),
  FOREIGN KEY (service_name) REFERENCES services(service_name)
);
```

---

### 10.3 SQLite 索引设计（v1）

设计原则：

- `v1` 只建立最小必需索引，不提前为假设查询过度优化。
- 索引围绕已确认的 Web UI / CLI 查询场景设计。
- 主键和唯一约束天然提供的索引不重复创建。
- 节点数量预计较少，`nodes` 表在 `v1` 不额外建立辅助索引。

核心查询场景：

- 任务历史页：最近任务列表、失败任务筛选、按服务筛选任务。
- 服务详情页：查看某服务最近任务、最近失败任务。
- 备份状态页：查看某服务最近备份、查看失败备份。
- 节点详情页：查看某节点相关任务历史。
- 服务列表页：默认只看 `is_declared = 1` 的服务，并可按 `runtime_status` 过滤。
- 任务详情页：按 `task_id` 读取全部步骤。

推荐索引：

`tasks`

- `idx_tasks_created_at` on (`created_at` DESC)
  用于最近任务列表和默认倒序分页。
- `idx_tasks_status_created_at` on (`status`, `created_at` DESC)
  用于失败任务排查和按状态过滤。
- `idx_tasks_service_created_at` on (`service_name`, `created_at` DESC)
  用于服务详情页查看任务历史。
- `idx_tasks_node_created_at` on (`node_id`, `created_at` DESC)
  用于节点详情页查看该节点任务历史。

`services`

- `idx_services_declared_runtime_status` on (`is_declared`, `runtime_status`)
  用于服务列表页默认过滤当前声明服务，并按运行状态筛选。

`task_steps`

- 不额外建立辅助索引。
- 原因：`PRIMARY KEY (task_id, step_name)` 已足够覆盖“按 `task_id` 读取步骤”的主要查询。

`backups`

- `idx_backups_service_finished_at` on (`service_name`, `finished_at` DESC)
  用于某服务最近备份列表。
- `idx_backups_status_finished_at` on (`status`, `finished_at` DESC)
  用于失败备份排查和备份状态页筛选。

`nodes`

- `v1` 不额外建立辅助索引。
- 原因：节点数量预计很少，`is_online` 过滤和节点列表查询不需要单独索引。

最小 DDL 补充：

```sql
CREATE INDEX idx_tasks_created_at
ON tasks(created_at DESC);

CREATE INDEX idx_tasks_status_created_at
ON tasks(status, created_at DESC);

CREATE INDEX idx_tasks_service_created_at
ON tasks(service_name, created_at DESC);

CREATE INDEX idx_tasks_node_created_at
ON tasks(node_id, created_at DESC);

CREATE INDEX idx_services_declared_runtime_status
ON services(is_declared, runtime_status);

CREATE INDEX idx_backups_service_finished_at
ON backups(service_name, finished_at DESC);

CREATE INDEX idx_backups_status_finished_at
ON backups(status, finished_at DESC);
```

---

### 10.4 ConnectRPC API（v1 草案）

设计原则：

- Web UI 和 CLI 完全共用同一套 Controller Public API。
- 所有 Controller Public API 在 `v1` 都必须能通过 Web UI 或 CLI 至少一种方式触达；常用运维动作要求两端都可触达。
- API 按领域拆服务，不做一个巨大总服务。
- 包名显式带版本，`v1` 先固定为第一版协议。
- 长时操作不走同步阻塞 RPC，而是创建任务后返回 `task_id`。
- agent 只主动连接 `controller`；`controller` 不主动反向连 agent。
- 鉴权统一使用 `Authorization: Bearer <token>`。
- 所有会改变 desired state 的写操作都必须最终落成 Git commit。
- 没有远程仓库时，写操作以本地 commit 成功为完成条件；有远程仓库时，写操作在本地 commit 成功后即可返回新的 revision，同时返回 push 结果和当前 repo sync 状态；若 push 失败，不回滚本地 `HEAD`。
- 所有创建执行类任务的接口都必须返回该任务绑定的 `repo_revision`。

建议的 proto 包：

- `composia.controller.v1`
- `composia.agent.v1`

#### Controller Public API

面向 Web UI 和 CLI 的 RPC。

建议拆分为以下服务：

- `ServiceService`
- `BackupRecordService`
- `TaskService`
- `NodeService`
- `RepoService`
- `SecretService`
- `SystemService`

##### `ServiceService`

职责：服务列表、服务详情、服务任务、服务备份，以及服务动作入口。

建议方法：

- `ListServices`
- `GetService`
- `GetServiceTasks`
- `GetServiceBackups`
- `DeployService`
- `UpdateService`
- `StopService`
- `RestartService`
- `BackupService`
- `UpdateServiceDNS`
- `MigrateService`

动作类方法语义：

- 不同步执行长任务。
- 成功时返回：`task_id`、初始 `status`、基础任务摘要、`repo_revision`。
- `BackupService` 支持可选 `data_names` 列表；为空时表示执行全部启用的 `backup.data[]`。
- `UpdateServiceDNS` 基于服务当前声明创建一个 `dns_update` 任务。
- `MigrateService` 接收目标 `node_id`，内部创建单个迁移任务；任务在自动步骤完成后可因等待人工验证或 repo 对账而进入 `awaiting_confirmation`；`result_revision` 仅在成功写回 repo 后产生。

##### `TaskService`

职责：任务历史、任务详情、重试、日志查看。

建议方法：

- `ListTasks`
- `GetTask`
- `RunTaskAgain`
- `TailTaskLogs`

方法语义：

- `ListTasks` 使用 cursor 分页。
- 核心过滤字段：`status`、`service_name`。
- `GetTask` 默认返回步骤摘要，不再额外强制拆一个 `GetTaskSteps`。
- `RunTaskAgain` 创建新的任务实例，不复用原 `task_id`；固定绑定调用时的最新 `HEAD` 作为新的 `repo_revision`。
- `RunTaskAgain` 的语义不是“重试同一次执行”，而是“基于当前系统状态再发起一次同类动作”；若原任务参数在当前配置下已不合法，则直接校验失败。
- `TailTaskLogs` 为服务端流式 RPC，支持客户端实时 tail 任务日志。

##### `BackupRecordService`

职责：全局备份记录查询和单条备份详情。

建议方法：

- `ListBackups`
- `GetBackup`

说明：

- `ListBackups` 支持按 `service_name`、`status`、时间窗口、`data_name` 过滤。
- 备份状态页默认使用 `ListBackups`，服务详情页仍可使用 `GetServiceBackups`。

##### `NodeService`

职责：节点列表和节点详情。

建议方法：

- `ListNodes`
- `GetNode`
- `GetNodeTasks`
- `ReloadNodeCaddy`
- `PruneNode`

说明：

- `ListNodes` 返回节点配置摘要和运行态快照聚合结果。
- `GetNode` 返回单节点详情，以及必要的最近状态摘要。
- `GetNodeTasks` 用于节点详情页查看最近任务历史。
- `ReloadNodeCaddy` 创建一个 `caddy_reload` 任务。
- `PruneNode` 创建一个 `prune` 任务，并接收清理范围参数。

##### `RepoService`

职责：仓库文件浏览和编辑。

建议方法：

- `GetRepoHead`
- `ListRepoFiles`
- `GetRepoFile`
- `ValidateRepo`
- `UpdateRepoFile`
- `ListRepoCommits`
- `SyncRepo`

说明：

- `v1` 虽然重点编辑 `docker-compose.yaml`、`.env`、`composia-meta.yaml`、`site_config.caddy`，但 API 层不限制为少数文件；原则上 repo 下文件都可读写。
- 读写模型采用“按指定路径读取/覆盖写回”，不做 patch-first 变更集模型。
- `GetRepoHead` 返回当前 branch、HEAD revision、是否配置远程、上次成功 pull 时间、是否处于 clean working tree，以及当前 repo sync 状态。
- `ValidateRepo` 执行 repo 级配置校验，返回结构化错误列表。
- `UpdateRepoFile` 请求至少应包含：`path`、`content`、`base_revision`、可选 `commit_message`。
- `UpdateRepoFile` 的服务端语义固定为：获取 repo lock、若配置远程则先 `fetch + fast-forward`、校验 `base_revision`、校验目标路径是否命中正在执行中的服务锁、写回文件、执行校验、生成 commit、若配置了远程则继续 push、返回新的 `commit_id`、push 结果和当前 repo sync 状态。
- `UpdateRepoFile` 在写文件 / 校验 / commit 之前的失败都必须回滚临时改动，不得留下 dirty working tree；若 commit 已成功但 push 失败，则不回滚本地 `HEAD`，repo 进入“远程未同步”状态。
- 某个服务存在 `pending`、`running` 或 `awaiting_confirmation` 任务时，命中该服务目录的 `UpdateRepoFile` 必须报冲突错误；修改其他服务或无关文件仍然允许，因此 repo `HEAD` 在长任务执行期间仍可能前进。
- `ListRepoCommits` 用于 Web UI 展示最近提交历史。
- `SyncRepo` 仅在配置远程仓库时可用，语义为 `fetch + fast-forward + re-parse`；如果 repo 当前不 clean，则直接失败。

##### `SecretService`

职责：读取和更新服务级 secrets 明文，由服务端负责重新加密写回 Git。

建议方法：

- `GetServiceSecretEnv`
- `UpdateServiceSecretEnv`

说明：

- `GetServiceSecretEnv` 返回解密后的 dotenv 文本，仅适用于单用户模式。
- `UpdateServiceSecretEnv` 请求至少应包含：`service_name`、`content`、`base_revision`、可选 `commit_message`。
- `UpdateServiceSecretEnv` 的写入事务与 `RepoService.UpdateRepoFile` 一致，但输出文件固定为服务目录下的 `.secret.env.enc`；如果该服务当前已有 `pending`、`running` 或 `awaiting_confirmation` 任务，则必须报服务冲突错误。
- `UpdateServiceSecretEnv` 成功后返回新的 `commit_id`，并在有远程仓库时返回 push 结果和当前 repo sync 状态。

##### `SystemService`

职责：系统级只读信息和手动运维入口。

建议方法：

- `GetSystemStatus`
- `GetCurrentConfig`

说明：

- `GetSystemStatus` 返回当前 repo HEAD、队列状态、数据库状态和版本信息。
- `GetCurrentConfig` 返回经过脱敏的运行配置摘要，供设置页和诊断页展示。

#### Controller Agent API

面向 agent 主动连接 `controller` 的 RPC。

建议拆分为以下服务：

- `AgentTaskService`
- `AgentReportService`
- `BundleService`

##### `AgentTaskService`

职责：agent 主动拉取待执行任务。

建议方法：

- `PullNextTask`

方法语义：

- 使用长轮询。
- 请求带上 `node_id` 和 agent 当前能力/版本摘要。
- 响应返回一个待执行任务，或在超时窗口内返回空结果。
- `v1` 任务队列对自动执行阶段采用全局串行，因此 `PullNextTask` 的调度逻辑会很保守；处于 `awaiting_confirmation` 的任务不阻塞后续任务下发。

##### `AgentReportService`

职责：心跳、任务状态、步骤状态、日志上报。

建议方法：

- `Heartbeat`
- `ReportTaskState`
- `ReportTaskStepState`
- `UploadTaskLogs`
- `ReportServiceStatus`

方法语义：

- `Heartbeat` 上报 `node_id`、`agent_version`、最近心跳时间，以及至少包含磁盘容量、剩余容量、Docker server version 的节点运行摘要。
- `ReportTaskState` 上报任务开始/结束状态。
- `ReportTaskStepState` 上报步骤开始/结束状态。
- `ReportServiceStatus` 上报本地服务运行状态摘要，用于刷新 `services.runtime_status`。

`UploadTaskLogs`：

- 使用流式 RPC。
- 每条日志事件至少带：`task_id`、`seq`、时间戳、日志内容。
- `controller` 记录最近已确认 `seq`。
- 流断开后，agent 重连并从未确认 `seq` 继续补传。
- `v1` 采用 `seq` 续传语义，而不是按字节偏移恢复。

##### `BundleService`

职责：agent 按任务需要下载服务包。

建议方法：

- `GetServiceBundle`

方法语义：

- bundle 标识采用 `git revision + service_name`。
- agent 先通过任务拿到需要的 revision 和服务名，再调用 `GetServiceBundle`。
- 响应采用文件流/数据流形式，避免把完整 bundle 塞进任务下发 RPC。
- `v1` 由 agent 主动下载 bundle；`controller` 不主动推送服务包。
- 如果配置了本地 `main` 节点，其 agent 也必须通过同一套 `PullNextTask + GetServiceBundle` 路径执行任务，不允许旁路本地文件系统特判。

#### 分页与过滤

- 列表型查询默认采用 cursor 分页。
- `ListTasks` 的核心过滤：`status`、`service_name`、可选 `node_id`、`type`。
- `ListServices` 可按 `runtime_status` 过滤。
- `GetServiceBackups` 可按 `status`、`data_name` 和时间窗口过滤。
- `ListBackups` 可按 `service_name`、`status`、`data_name`、时间窗口过滤。

#### 日志接口

- agent -> controller：流式上传。
- controller -> Web/CLI：`TaskService.TailTaskLogs` 流式 tail。
- 日志 tail 支持断线重连，并按最后已消费的 `seq` 继续。
- 非流式日志查看可由 `GetTask` 返回基础 `log_path` 和日志元信息，必要时后续补普通读取 RPC。

#### 认证模型

- Web UI / CLI 使用 `controller.cli_tokens[]` 对应的 admin token。
- agent 使用 `controller.nodes[]` 中与 `node_id` 对应的 node token。
- 两类调用者共用 `Authorization: Bearer <token>` 头，但服务端按 token 来源语义区分权限。

#### 典型调用链

Web / CLI 发起部署：

1. 调用 `ServiceService.DeployService`
2. `controller` 创建任务并返回 `task_id`
3. agent 通过 `AgentTaskService.PullNextTask` 拉到任务
4. agent 通过 `BundleService.GetServiceBundle` 下载对应 revision 的服务包
5. agent 执行任务并通过 `AgentReportService` 持续上报步骤和日志
6. 客户端通过 `TaskService.GetTask` / `TailTaskLogs` 查看进度

Web 在线编辑文件：

1. 调用 `RepoService.GetRepoHead` 和 `RepoService.GetRepoFile`
2. 用户修改内容
3. 调用 `RepoService.UpdateRepoFile`
4. `controller` 若配置远程则先执行 `fetch + fast-forward`，然后写回、校验、`commit`，并在有远程时执行 `push`
5. 返回新的 `commit_id`，并触发必要的重新解析或后续任务

#### v1 暂不做

- 单一 `CreateTask` 通用入口
- `controller` 主动推送任务到 agent
- agent 反向暴露固定 RPC 服务给 `controller`
- Web/CLI 与 agent 使用不同协议栈
- 复杂权限模型或细粒度 RBAC

### 11. Web UI 页面

核心页面：服务列表 / 服务详情 / 节点列表 / 备份状态 / 任务历史 / 文档页 / 设置页。

`v1` 页面上必须可直接完成：

- 在线编辑仓库文件（`docker-compose.yaml`、`.env`、`composia-meta.yaml`、Caddy 片段）。
- 在线编辑服务级 secrets（服务端负责解密/重新加密 `.secret.env.enc`）。
- 每次在线编辑成功后展示对应的 Git commit 结果、最新 revision，以及远程模式下的 push 结果。
- 部署控制（部署、停止、重启、更新、迁移）。
- 节点运维（节点状态、磁盘、Docker 信息、最近心跳）。

---

### 12. CLI 命令（v1）

设计原则：

- CLI 作为运维入口，覆盖 SSH / 脚本化场景的常用操作。
- 常用 Controller Public API 必须有对应 CLI 子命令。
- `v1` 不内建交互式文本编辑器；文本类修改通过 `--from-file`、stdin 或外部 `$EDITOR` 包装完成。

已确认核心：

- `service list/get/deploy/stop/restart/update/backup/migrate/logs`
- `task list/get/run-again/logs`
- `backup list/get`
- `node list/get/tasks/reload-caddy/prune`
- `repo head/files/get/update/history/sync`
- `secret get/update`
- `system status`
- `caddy reload`
- `dns update`

---

### 13. 长任务与 Repo 并发规则（v1 补充）

- 服务级冲突判定优先于 repo 写入：某服务存在 `pending`、`running` 或 `awaiting_confirmation` 任务时，不允许再创建该服务的新任务，也不允许把任何触及该服务目录的修改写入 repo。
- Web UI / CLI 可以保留未提交草稿，但草稿只能存在于客户端或独立 draft store 中；`controller.repo_dir` 在空闲状态下必须保持 clean，不能承载“先改文件、稍后再提交”的草稿态。
- 对其他服务或无关文件的正式修改仍然允许，因此长任务执行期间 repo `HEAD` 仍可能因为跨服务提交、同步远程或其他系统写操作而前进。
- `v1` 当前主要需要处理该问题的长任务是 `migrate`。`migrate` 的自动执行部分始终绑定任务创建时的 `repo_revision`；agent 只消费该 revision 对应的 bundle，不受后续 Git 提交影响。
- `migrate` 进入 `persist_repo` 时，`controller` 必须重新获取 repo lock，并比较当前 `HEAD` 与任务初始 `repo_revision`。
- 如果 `HEAD` 没有变化，直接按正常流程把该服务的最终 `node` 写回 repo、commit，并在配置了远程仓库时继续 push。
- 如果 `HEAD` 已变化，但变化仅涉及其他服务或无关文件，则允许在最新 `HEAD` 上补写该服务最终状态，再生成新的 commit。
- 如果最新变更触及该服务目录、使该服务声明不再有效，或触及迁移所依赖的相关全局配置，则视为冲突；`controller` 不得自动覆盖这些变更，也不得回退 repo。
- 发生上述冲突时，运行态迁移结果保持不变，但 repo 写回动作停止；任务进入 `awaiting_confirmation`，并向用户明确暴露“运行态已迁移，但 repo 仍需人工对账”。
