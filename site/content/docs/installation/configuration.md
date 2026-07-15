---
title: "Configuration"
date: '2026-05-26T00:00:00+08:00'
weight: 40
---

This page covers installation-level configuration: controller config, agent config, web environment variables, and age key setup.

Service definitions live in `composia-meta.yaml`. See [Service Guide](/docs/guide/service/) for that file.

## Config file shape

The controller and agent use the same YAML file format. A file may contain either section or both:

```yaml
controller:
  # controller settings

agent:
  # agent settings
```

At least one of `controller` or `agent` must be present.

When the same config file contains both sections, the local agent is treated as the built-in node:

- `agent.node_id` must be `main`.
- `controller.nodes` must include an entry with `id: main`.
- `controller.repo_dir` and `agent.repo_dir` must not be the same path.

## Full config template

This template shows every supported installation-level key. It is a shape reference, not a copy-paste default. Remove sections you do not use, remove empty list items, and use either inline values or `_file` values for each secret-like field.

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

Do not keep empty list items such as `controller_headers` with an empty `name`. They are shown only to document the supported object shape.

The web access token and main agent token must be different.

## Age key setup

`controller.secrets` is optional. Configure it only if you use Composia-managed encrypted secrets.

When `controller.secrets` is configured, `identity_file` is required. `recipient_file` is optional. If it is omitted, Composia derives the recipient from the private key.

Generate a private key:

```bash
age-keygen -o age-identity.key
```

Optional recipient file:

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

Use the private key in config:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
```

Or use both files:

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
```

`armor` is optional and defaults to `true`.

## Controller config reference

### Required keys

| Key | Type | Description |
|-----|------|-------------|
| `listen_addr` | `string` | Controller listen address, for example `":7001"` or `"127.0.0.1:7001"`. |
| `repo_dir` | `string` | Desired-state Git repository path. |
| `state_dir` | `string` | Controller state path. |
| `log_dir` | `string` | Task log directory. |
| `nodes` | `[]object` | Configured agent nodes. The key must be present, even if empty. |

### Optional top-level keys

| Key | Type | Description |
|-----|------|-------------|
| `access_tokens` | `[]object` | API tokens for web UI, CLI, and external clients. |
| `backup` | `object` | Global backup defaults. |
| `git` | `object` | Desired-state repository remote sync. |
| `notifications` | `object` | Alertmanager, SMTP, and Telegram notifications. |
| `dns` | `object` | DNS provider credentials. |
| `rustic` | `object` | Rustic maintenance settings. |
| `secrets` | `object` | Age encryption settings. |
| `updates` | `object` | Image update defaults and forge API auth. |
| `auto_deploy` | `object` | Global auto-deploy toggles. |

### `nodes[]`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `id` | `string` | Yes | Unique node ID. |
| `display_name` | `string` | No | Name shown in UI. |
| `enabled` | `bool` | No | Disable a node without removing it. |
| `public_ipv4` | `string` | No | Public IPv4 used by DNS workflows. |
| `public_ipv6` | `string` | No | Public IPv6 used by DNS workflows. |
| `token` | `string` | Yes* | Agent auth token. |
| `token_file` | `string` | No | Read the token from a file. |

*Use either `token` or `token_file`, not both.

### `access_tokens[]`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Token name. |
| `token` | `string` | Yes* | Token value. |
| `token_file` | `string` | No | Read the token from a file. |
| `enabled` | `bool` | No | Disable a token without removing it. |
| `comment` | `string` | No | Administrative note. |

Access tokens must not duplicate node tokens or other access tokens.

### `git`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `remote_url` | `string` | No | Git remote URL. |
| `branch` | `string` | No | Branch to sync. |
| `pull_interval` | `string` | Cond. | Required when `remote_url` is set. |
| `author_name` | `string` | No | Commit author name for controller writes. |
| `author_email` | `string` | No | Commit author email. |
| `auth.username` | `string` | No | Git username. |
| `auth.token` | `string` | No | Git token. |
| `auth.token_file` | `string` | No | Read Git token from a file. |

### `secrets`

This whole section is optional. If the section is present, these rules apply:

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Yes | Must be `age`. |
| `identity_file` | `string` | Yes | Age private key path. |
| `recipient_file` | `string` | No | Age recipient file path. If omitted, recipient is derived from `identity_file`. |
| `armor` | `bool` | No | ASCII armor encrypted output. Defaults to `true`. |

### `backup`

| Key | Type | Description |
|-----|------|-------------|
| `default_schedule` | `string` | Default cron schedule for service backups. |

### `updates`

| Key | Type | Description |
|-----|------|-------------|
| `default_check_schedule` | `string` | Default cron schedule for image update checks. |
| `auto_apply` | `bool` | Apply updates automatically by default. |
| `backup_before_update` | `bool` | Back up data before applying updates. |
| `digest_pin` | `bool` | Pin images by digest. |
| `semver.default_allow` | `[]string` | Allowed semver bump levels: `patch`, `minor`, `major`. |
| `forge_auth.github` | `object` or `[]object` | GitHub API auth. |
| `forge_auth.gitlab` | `object` or `[]object` | GitLab API auth. |
| `forge_auth.forgejo` | `object` or `[]object` | Forgejo API auth. |

Each forge auth entry supports:

| Key | Type | Description |
|-----|------|-------------|
| `url` | `string` | Forge base URL. |
| `token` | `string` | API token. |
| `token_file` | `string` | Read API token from a file. |
| `api_url` | `string` | API URL override. |

### `auto_deploy`

| Key | Type | Description |
|-----|------|-------------|
| `infra` | `bool` | Auto-deploy infrastructure services after Git changes. |
| `services` | `bool` | Auto-deploy regular services after Git changes. |

### `dns`

| Provider key | Credential keys | Common keys |
|--------------|-----------------|-------------|
| `cloudflare` | `api_token`, `api_token_file` | `zones` |
| `alidns` | `access_key_id`, `access_key_id_file`, `access_key_secret`, `access_key_secret_file`, `security_token`, `security_token_file`, `region_id` | `zones` |
| `dnspod` | `secret_id`, `secret_id_file`, `secret_key`, `secret_key_file`, `session_token`, `session_token_file`, `region` | `zones` |
| `route53` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `session_token`, `session_token_file`, `region`, `profile`, `hosted_zone_id` | `zones` |
| `huaweicloud` | `access_key_id`, `access_key_id_file`, `secret_access_key`, `secret_access_key_file`, `region_id` | `zones` |

### `rustic`

| Key | Type | Description |
|-----|------|-------------|
| `main_nodes` | `[]string` | Node IDs that run Rustic operations. Each must reference `controller.nodes`. |
| `maintenance.forget_schedule` | `string` | Cron schedule for `rustic forget`. |
| `maintenance.prune_schedule` | `string` | Cron schedule for `rustic prune`. |

### `notifications.alertmanager`

| Key | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Enabled by default when section exists. |
| `listen_path` | `string` | Webhook path. Defaults to `/api/v1/alerts`. Must start with `/`. |

### `notifications.smtp`

| Key | Type | Required when enabled | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | No | Enabled by default when section exists. |
| `host` | `string` | Yes | SMTP host. |
| `port` | `int` | Yes | SMTP port, 1 to 65535. |
| `encryption` | `string` | No | `none`, `starttls`, or `ssl_tls`. Defaults to `starttls`. |
| `username` | `string` | No | SMTP username. |
| `password` | `string` | No | SMTP password. |
| `password_file` | `string` | No | Read password from a file. |
| `from` | `string` | Yes | Sender address. |
| `to` | `[]string` | Yes | Recipient list. |
| `on` | `[]string` | No | Notification event filters. |
| `task_sources` | `[]string` | No | Task source filters: `web`, `cli`, `others`, `schedule`, `system`, `auto_deploy`. |

### `notifications.telegram`

| Key | Type | Required when enabled | Description |
|-----|------|-----------------------|-------------|
| `enabled` | `bool` | No | Enabled by default when section exists. |
| `bot_token` | `string` | Yes* | Telegram bot token. |
| `bot_token_file` | `string` | No | Read bot token from a file. |
| `chat_id` | `string` | Yes | Target chat ID. |
| `on` | `[]string` | No | Notification event filters. |
| `task_sources` | `[]string` | No | Task source filters. |

## Agent config reference

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `controller_addr` | `string` | Yes | Controller URL reachable from the agent. |
| `controller_grpc` | `bool` | No | Use gRPC instead of Connect over HTTP. |
| `controller_headers` | `[]object` | No | Extra HTTP headers sent to the controller. |
| `node_id` | `string` | Yes | This agent's node ID. Must match `controller.nodes[].id`. |
| `token` | `string` | Yes* | Node token matching the controller config. |
| `token_file` | `string` | No | Read node token from a file. |
| `repo_dir` | `string` | Yes | Agent service repository path. |
| `state_dir` | `string` | Yes | Agent state directory. |
| `caddy` | `object` | No | Agent-side Caddy settings. |

*Use either `token` or `token_file`, not both.

### `controller_headers[]`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | HTTP header name. Header names are deduplicated case-insensitively. |
| `value` | `string` | Yes* | Header value. |
| `value_file` | `string` | No | Read header value from a file. |

### `caddy`

| Key | Type | Description |
|-----|------|-------------|
| `generated_dir` | `string` | Generated Caddy config directory. Defaults to `<state_dir>/caddy/generated`. |

## Web environment variables

The web server reads environment variables. In Docker Compose these are set through `.env`.

| Variable | Required | Description |
|----------|----------|-------------|
| `WEB_CONTROLLER_ADDR` | Yes | Controller address from the web server process. In Docker Compose: `http://controller:7001`. |
| `WEB_BROWSER_CONTROLLER_ADDR` | Yes | Controller address from the browser. |
| `WEB_CONTROLLER_ACCESS_TOKEN` | Yes | Controller access token. Must match `controller.access_tokens[].token`. |
| `WEB_CONTROLLER_HEADERS` | No | JSON object of extra headers sent by the web server when calling the controller. |
| `WEB_LOGIN_USERNAME` | Yes | Web login username. |
| `WEB_LOGIN_PASSWORD_HASH` | Yes | Argon2 password hash. |
| `WEB_SESSION_SECRET` | Yes | Random session signing secret. |
| `ORIGIN` | Deployment-dependent | Public origin of the web server. |
| `HOST` | No | Host bind address. |
| `PORT` | No | Web server port. |

## Inline values and `_file` values

Many secret-like fields support both inline values and file references. Examples:

- `token` / `token_file`
- `password` / `password_file`
- `api_token` / `api_token_file`
- `value` / `value_file`

Use one form only. If both are set, startup fails.
