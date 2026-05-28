---
title: "配置"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

本页涵盖安装级别的配置：控制器配置、agent 配置、Web 环境变量和 age 密钥设置。

服务定义位于 `composia-meta.yaml` 中。请参阅[服务指南](/docs/guide/service/)了解该文件。

## 配置文件结构

控制器和 agent 使用相同的 YAML 文件格式。一个文件可以包含其中一部分或两部分：

```yaml
controller:
  # 控制器设置

agent:
  # agent 设置
```

`controller` 或 `agent` 中至少必须存在一个。

当同一个配置文件包含两部分时，本地 agent 被视为内置节点：

- `agent.node_id` 必须为 `main`。
- `controller.nodes` 必须包含一个 `id: main` 的条目。
- `controller.repo_dir` 和 `agent.repo_dir` 不能是同一路径。

## 完整配置模板

此模板展示了每个受支持的安装级别键。它是一个结构参考，而不是复制粘贴的默认配置。请删除您不使用的部分，删除空列表项，并对每个类似密钥的字段使用内联值或 `_file` 值。

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"

  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      token_file: ""
      enabled: true
      comment: "Web UI access token"

  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      public_ipv4: ""
      public_ipv6: ""
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
      token_file: ""

  git:
    remote_url: ""
    branch: "main"
    pull_interval: ""
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: ""
      token: ""
      token_file: ""

  backup:
    default_schedule: ""

  updates:
    default_check_schedule: ""
    auto_apply: false
    backup_before_update: true
    digest_pin: false
    semver:
      default_allow:
        - patch
        - minor
    forge_auth:
      github:
        url: "https://github.com"
        token: ""
        token_file: ""
        api_url: "https://api.github.com"
      gitlab:
        url: "https://gitlab.com"
        token: ""
        token_file: ""
        api_url: "https://gitlab.com/api/v4"
      forgejo:
        url: "https://forgejo.example.com"
        token: ""
        token_file: ""
        api_url: ""

  auto_deploy:
    infra: false
    services: false

  dns:
    cloudflare:
      api_token: ""
      api_token_file: ""
      zones: []
    alidns:
      access_key_id: ""
      access_key_id_file: ""
      access_key_secret: ""
      access_key_secret_file: ""
      security_token: ""
      security_token_file: ""
      region_id: ""
      zones: []
    dnspod:
      secret_id: ""
      secret_id_file: ""
      secret_key: ""
      secret_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      zones: []
    route53:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      session_token: ""
      session_token_file: ""
      region: ""
      profile: ""
      hosted_zone_id: ""
      zones: []
    huaweicloud:
      access_key_id: ""
      access_key_id_file: ""
      secret_access_key: ""
      secret_access_key_file: ""
      region_id: ""
      zones: []

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: ""
      prune_schedule: ""

  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: ""
    armor: true

  notifications:
    alertmanager:
      enabled: true
      listen_path: "/api/v1/alerts"
    smtp:
      enabled: false
      host: ""
      port: 587
      encryption: starttls
      username: ""
      password: ""
      password_file: ""
      from: ""
      to: []
      on: []
      task_sources: []
    telegram:
      enabled: false
      bot_token: ""
      bot_token_file: ""
      chat_id: ""
      on: []
      task_sources: []

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  controller_headers:
    - name: ""
      value: ""
      value_file: ""
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  token_file: ""
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: ""
```

不要保留空的列表项，例如包含空 `name` 的 `controller_headers`。它们仅用于展示支持的对象结构。

Web 访问令牌和主 agent 令牌必须不同。

## Age 密钥设置

`controller.secrets` 是可选的。仅当使用 Composia 管理的加密密钥时才配置它。

当配置了 `controller.secrets` 时，`identity_file` 是必填的。`recipient_file` 是可选的。如果省略，Composia 从私钥派生接收者。

生成私钥：

```bash
age-keygen -o age-identity.key
```

可选的接收者文件：

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

在配置中使用私钥：

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

或同时使用两个文件：

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` 是可选的，默认为 `true`。

## 控制器配置参考

### 必填键

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `listen_addr` | `string` | 控制器监听地址，例如 `":7001"` 或 `"127.0.0.1:7001"`。 |
| `repo_dir` | `string` | 期望状态 Git 仓库路径。 |
| `state_dir` | `string` | 控制器状态路径。 |
| `log_dir` | `string` | 任务日志目录。 |
| `nodes` | `[]object` | 已配置的 agent 节点。该键必须存在，即使为空。 |

### 可选顶级键

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `access_tokens` | `[]object` | 用于 Web UI、CLI 和外部客户端的 API 令牌。 |
| `backup` | `object` | 全局备份默认值。 |
| `git` | `object` | 期望状态仓库的远程同步。 |
| `notifications` | `object` | Alertmanager、SMTP 和 Telegram 通知。 |
| `dns` | `object` | DNS 提供商凭据。 |
| `rustic` | `object` | Rustic 维护设置。 |
| `secrets` | `object` | Age 加密设置。 |
| `updates` | `object` | 镜像更新默认值和 forge API 认证。 |
| `auto_deploy` | `object` | 全局自动部署开关。 |

### `nodes[]`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `id` | `string` | 是 | 唯一的节点 ID。 |
| `display_name` | `string` | 否 | UI 中显示的名称。 |
| `enabled` | `bool` | 否 | 禁用节点而不删除配置。 |
| `public_ipv4` | `string` | 否 | DNS 工作流使用的公网 IPv4。 |
| `public_ipv6` | `string` | 否 | DNS 工作流使用的公网 IPv6。 |
| `token` | `string` | 是* | Agent 认证令牌。 |
| `token_file` | `string` | 否 | 从文件读取令牌。 |

*使用 `token` 或 `token_file`，不能同时使用。

### `access_tokens[]`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 令牌名称。 |
| `token` | `string` | 是* | 令牌值。 |
| `token_file` | `string` | 否 | 从文件读取令牌。 |
| `enabled` | `bool` | 否 | 禁用令牌而不删除配置。 |
| `comment` | `string` | 否 | 管理备注。 |

访问令牌不能与节点令牌或其他访问令牌重复。

### `git`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `remote_url` | `string` | 否 | Git 远程 URL。 |
| `branch` | `string` | 否 | 要同步的分支。 |
| `pull_interval` | `string` | 条件 | 当 `remote_url` 设置时必填。 |
| `author_name` | `string` | 否 | 控制器写入的提交作者名。 |
| `author_email` | `string` | 否 | 提交作者邮箱。 |
| `auth.username` | `string` | 否 | Git 用户名。 |
| `auth.token` | `string` | 否 | Git 令牌。 |
| `auth.token_file` | `string` | 否 | 从文件读取 Git 令牌。 |

### `secrets`

整个部分是可选的。如果该部分存在，以下规则适用：

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | 必须为 `age`。 |
| `identity_file` | `string` | 是 | Age 私钥路径。 |
| `recipient_file` | `string` | 否 | Age 接收者文件路径。如果省略，接收者从 `identity_file` 派生。 |
| `armor` | `bool` | 否 | ASCII 封装加密输出。默认为 `true`。 |

### `backup`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `default_schedule` | `string` | 服务备份的默认 cron 计划。 |

### `updates`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `default_check_schedule` | `string` | 镜像更新检查的默认 cron 计划。 |
| `auto_apply` | `bool` | 默认自动应用更新。 |
| `backup_before_update` | `bool` | 在应用更新前备份数据。 |
| `digest_pin` | `bool` | 通过摘要锁定镜像。 |
| `semver.default_allow` | `[]string` | 允许的 semver 升级级别：`patch`、`minor`、`major`。 |
| `forge_auth.github` | `object` 或 `[]object` | GitHub API 认证。 |
| `forge_auth.gitlab` | `object` 或 `[]object` | GitLab API 认证。 |
| `forge_auth.forgejo` | `object` 或 `[]object` | Forgejo API 认证。 |

每个 forge 认证条目支持：

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `url` | `string` | Forge 基础 URL。 |
| `token` | `string` | API 令牌。 |
| `token_file` | `string` | 从文件读取 API 令牌。 |
| `api_url` | `string` | API URL 覆盖。 |

### `auto_deploy`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `infra` | `bool` | Git 更改后自动部署基础设施服务。 |
| `services` | `bool` | Git 更改后自动部署普通服务。 |

### `dns`

| 提供商键 | 凭据键 | 通用键 |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`、`api_token_file` | `zones` |
| `alidns` | `access_key_id`、`access_key_id_file`、`access_key_secret`、`access_key_secret_file`、`security_token`、`security_token_file`、`region_id` | `zones` |
| `dnspod` | `secret_id`、`secret_id_file`、`secret_key`、`secret_key_file`、`session_token`、`session_token_file`、`region` | `zones` |
| `route53` | `access_key_id`、`access_key_id_file`、`secret_access_key`、`secret_access_key_file`、`session_token`、`session_token_file`、`region`、`profile`、`hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`、`access_key_id_file`、`secret_access_key`、`secret_access_key_file`、`region_id` | `zones` |

### `rustic`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `main_nodes` | `[]string` | 运行 Rustic 操作的节点 ID 列表。每个都必须引用 `controller.nodes`。 |
| `maintenance.forget_schedule` | `string` | `rustic forget` 的 cron 计划。 |
| `maintenance.prune_schedule` | `string` | `rustic prune` 的 cron 计划。 |

### `notifications.alertmanager`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `enabled` | `bool` | 当该部分存在时默认启用。 |
| `listen_path` | `string` | Webhook 路径。默认为 `/api/v1/alerts`。必须以 `/` 开头。 |

### `notifications.smtp`

| 键 | 类型 | 启用时必填 | 描述 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 当该部分存在时默认启用。 |
| `host` | `string` | 是 | SMTP 主机。 |
| `port` | `int` | 是 | SMTP 端口，1 到 65535。 |
| `encryption` | `string` | 否 | `none`、`starttls` 或 `ssl_tls`。默认为 `starttls`。 |
| `username` | `string` | 否 | SMTP 用户名。 |
| `password` | `string` | 否 | SMTP 密码。 |
| `password_file` | `string` | 否 | 从文件读取密码。 |
| `from` | `string` | 是 | 发件人地址。 |
| `to` | `[]string` | 是 | 收件人列表。 |
| `on` | `[]string` | 否 | 通知事件过滤器。 |
| `task_sources` | `[]string` | 否 | 任务来源过滤器：`web`、`cli`、`others`、`schedule`、`system`。 |

### `notifications.telegram`

| 键 | 类型 | 启用时必填 | 描述 |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | 否 | 当该部分存在时默认启用。 |
| `bot_token` | `string` | 是* | Telegram 机器人令牌。 |
| `bot_token_file` | `string` | 否 | 从文件读取机器人令牌。 |
| `chat_id` | `string` | 是 | 目标聊天 ID。 |
| `on` | `[]string` | 否 | 通知事件过滤器。 |
| `task_sources` | `[]string` | 否 | 任务来源过滤器。 |

## Agent 配置参考

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `controller_addr` | `string` | 是 | Agent 可访问的控制器 URL。 |
| `controller_grpc` | `bool` | 否 | 使用 gRPC 而非基于 HTTP 的 Connect。 |
| `controller_headers` | `[]object` | 否 | 发送给控制器的额外 HTTP 头。 |
| `node_id` | `string` | 是 | 此 agent 的节点 ID。必须与 `controller.nodes[].id` 匹配。 |
| `token` | `string` | 是* | 与控制器配置匹配的节点令牌。 |
| `token_file` | `string` | 否 | 从文件读取节点令牌。 |
| `repo_dir` | `string` | 是 | Agent 服务仓库路径。 |
| `state_dir` | `string` | 是 | Agent 状态目录。 |
| `caddy` | `object` | 否 | Agent 端 Caddy 设置。 |

*使用 `token` 或 `token_file`，不能同时使用。

### `controller_headers[]`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | HTTP 头名称。头名称按不区分大小写去重。 |
| `value` | `string` | 是* | 头值。 |
| `value_file` | `string` | 否 | 从文件读取头值。 |

### `caddy`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `generated_dir` | `string` | 生成的 Caddy 配置目录。默认为 `<state_dir>/caddy/generated`。 |

## Web 环境变量

Web 服务器读取环境变量。在 Docker Compose 中，这些通过 `.env` 设置。

| 变量 | 必填 | 描述 |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | 是 | 从 Web 服务器进程访问的控制器地址。在 Docker Compose 中：`http://controller:7001`。 |
| `WEB_BROWSER_CONTROLLER_ADDR` | 是 | 从浏览器访问的控制器地址。 |
| `WEB_CONTROLLER_ACCESS_TOKEN` | 是 | 控制器访问令牌。必须与 `controller.access_tokens[].token` 匹配。 |
| `WEB_CONTROLLER_HEADERS` | 否 | Web 服务器调用控制器时发送的额外 HTTP 头的 JSON 对象。 |
| `WEB_LOGIN_USERNAME` | 是 | Web 登录用户名。 |
| `WEB_LOGIN_PASSWORD_HASH` | 是 | Argon2 密码哈希。 |
| `WEB_SESSION_SECRET` | 是 | 随机会话签名密钥。 |
| `ORIGIN` | 视部署而定 | Web 服务器的公开来源。 |
| `HOST` | 否 | 主机绑定地址。 |
| `PORT` | 否 | Web 服务器端口。 |

## 内联值与 `_file` 值

许多类似密钥的字段同时支持内联值和文件引用。例如：

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

只能使用一种形式。如果两者都设置，启动将失败。
