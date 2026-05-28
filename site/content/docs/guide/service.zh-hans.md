---
title: "服务配置"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

每个服务位于控制器仓库的一个目录中。服务目录包含 `composia-meta.yaml` 和一个或多个 Docker Compose 文件。

最小服务：

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

在默认行为下，Composia 会在同一目录中查找 `docker-compose.yaml`。

## 顶级键

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 唯一的服务名称。 |
| `project_name` | `string` | 否 | Docker Compose 项目名称覆盖。默认为规范化后的服务名称。 |
| `compose_files` | `[]string` | 否 | 相对于服务目录的 Compose 文件路径。 |
| `enabled` | `bool` | 否 | 服务是否激活。默认为 `true`。 |
| `nodes` | `[]string` | 是 | 目标节点 ID 列表。每个都必须存在于 `controller.nodes` 中。 |
| `infra` | `object` | 否 | 声明此服务为 Caddy、Rustic 或仅配置型基础设施。 |
| `network` | `object` | 否 | Caddy 和 DNS 设置。 |
| `update` | `object` | 否 | 镜像更新设置。 |
| `data_protect` | `object` | 否 | 备份和恢复数据定义。 |
| `backup` | `object` | 否 | 受保护数据的定时备份。 |
| `migrate` | `object` | 否 | 启用迁移的受保护数据。 |
| `auto_deploy` | `bool` | 否 | 仓库更改后自动部署此服务。 |

`compose_files` 条目必须是相对路径，必须保持在服务目录内，且不能重复。

## 基础设施服务

### `infra.caddy`

声明仓库的 Caddy 基础设施服务。

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服务名称。默认为 `caddy`。 |
| `config_dir` | `string` | Caddy 配置目录。默认为 `/etc/caddy`。 |

只能有一个服务被声明为 Caddy 基础设施。

### `infra.rustic`

声明仓库的 Rustic 基础设施服务。

```yaml
infra:
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-version
      - "2"
```

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `compose_service` | `string` | Compose 服务名称。默认为 `rustic`。 |
| `profile` | `string` | Rustic 配置文件名称。 |
| `data_protect_dir` | `string` | 数据保护工作流使用的目录。 |
| `init_args` | `[]string` | 传递给 `rustic init` 的额外参数。空条目会被拒绝。 |

只能有一个服务被声明为 Rustic 基础设施。

### `infra.config`

声明一个仅配置型基础设施服务。

```yaml
infra:
  config: {}
```

仅配置型服务不能与 `infra.caddy` 或 `infra.rustic` 组合。其 `data_protect` 操作只能使用 `files.copy`。

## 网络

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `enabled` | `bool` | 否 | 启用 Caddy 管理。默认为 `false`。 |
| `source` | `string` | 条件 | 相对于服务目录的 Caddyfile 路径。启用时必填。 |

### `network.dns`

```yaml
network:
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    value: 203.0.113.10
    proxied: true
    ttl: 120
    comment: 由 Composia 管理
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | `cloudflare`、`alidns`、`dnspod`、`route53` 或 `huaweicloud`。 |
| `hostname` | `string` | 是 | DNS 主机名。 |
| `record_type` | `string` | 否 | 空、`A`、`AAAA` 或 `CNAME`。 |
| `value` | `string` | 否 | DNS 记录值。多节点服务应显式设置此项。 |
| `proxied` | `bool` | 否 | 提供商特定的代理开关，目前与 Cloudflare 相关。 |
| `ttl` | `uint32` | 否 | DNS TTL。 |
| `comment` | `string` | 否 | DNS 记录备注。 |

## 镜像更新

```yaml
update:
  enabled: true
  auto_apply: false
  check_schedule: "0 */6 * * *"
  backup_before_update: true
  digest_pin: false
  backup_data:
    - name: db
      enabled: true
  discovery_sources:
    upstream:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    app:
      image: ghcr.io/example/app
      current:
        env:
          file: .env
          key: APP_VERSION
      discovery: upstream
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `enabled` | `bool` | 为此服务启用更新检查。 |
| `auto_apply` | `bool` | 自动应用检测到的更新。 |
| `check_schedule` | `string` | 更新检查的 cron 计划。 |
| `backup_before_update` | `bool` | 在应用更新之前运行备份。 |
| `backup_data` | `[]object` | 更新前要备份的受保护数据项。 |
| `digest_pin` | `bool` | 通过摘要锁定镜像。 |
| `discovery_sources` | `map[string]object` | 可复用的发现源。命名源不能引用另一个源。 |
| `images` | `map[string]object` | 每镜像更新定义。 |

### `update.backup_data[]`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `name` | `string` | 受保护的数据项名称。 |
| `enabled` | `bool` | 包含或排除此项。 |

### `update.images.<name>`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `image` | `string` | 是 | 镜像仓库。 |
| `auto_apply` | `bool` | 否 | 每镜像自动应用覆盖。 |
| `check_schedule` | `string` | 否 | 每镜像检查计划。 |
| `backup_before_update` | `bool` | 否 | 每镜像备份开关。 |
| `digest_pin` | `bool` | 否 | 每镜像摘要锁定开关。 |
| `current` | `object` | 是 | 当前版本来源。 |
| `discovery` | `object` 或 `string` | 是 | 发现配置或命名发现源引用。 |
| `filter` | `object` | 条件 | 必填，除非发现模式为 `digest`。 |

### `current`

指定以下之一：

| 键 | 描述 |
|-----|-------------|
| `tag` | 静态当前标签。 |
| `env.file` + `env.key` | 从环境变量文件读取当前标签。`file` 必须是相对路径且保持在服务目录内。 |
| `yaml.file` + `yaml.path` | 从 YAML 文件读取当前标签。`file` 必须是相对路径且保持在服务目录内。 |

### `discovery`

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `sources` | `[]object` | 至少一个源。 |
| `combine` | `string` | 空、`merge` 或 `first_success`。 |
| `include_prerelease` | `bool` | 包含预发布版本。 |

发现源类型：

| 类型 | 必填键 | 备注 |
|------|---------------|-------|
| `auto` | 无 | `repo_url` 可选，设置时必须为有效 URL。必须是唯一源。 |
| `probe` | 无 | 当存在过滤器时需要 `semver` 过滤器。 |
| `registry` | 无 | 注册表标签发现。 |
| `digest` | 无 | 必须是唯一源。必须省略 `filter`。 |
| `github` | `repo` | `repo` 格式为 `owner/repo`。 |
| `gitlab` | `project` | GitLab 项目 ID 或路径。 |
| `forgejo` | `repo` | `repo` 格式为 `owner/repo`。 |

### `filter`

| 类型 | 必填键 | 备注 |
|------|---------------|-------|
| `semver` | 无 | `allow` 可包含 `patch`、`minor`、`major`。 |
| `date` | `format` | 用于解析标签的日期格式。 |
| `regex` | `pattern`、`order` | `order` 必须为 `numeric` 或 `lexicographic`。 |
| `latest` | 无 | 使用最新候选版本。 |

## 数据保护

```yaml
data_protect:
  data:
    - name: db
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres
    - name: uploads
      backup:
        strategy: files.copy_after_stop
        include:
          - ./uploads
      restore:
        strategy: files.copy
        include:
          - ./uploads
```

### `data_protect.data[]`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 唯一的数据项名称。 |
| `backup` | `object` | 否 | 备份操作。 |
| `restore` | `object` | 否 | 恢复操作。 |

### 数据操作

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `strategy` | `string` | 是 | `files.copy`、`files.copy_after_stop`、`database.pgdumpall` 或 `database.pgimport`。 |
| `service` | `string` | 条件 | `database.*` 策略必填。Compose 服务名称。 |
| `include` | `[]string` | 条件 | `files.*` 策略必填。 |

## 备份

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必须引用具有备份操作的 `data_protect.data[].name`。 |
| `provider` | `string` | 否 | 备份提供者名称。 |
| `enabled` | `bool` | 否 | 启用或禁用此备份条目。 |
| `schedule` | `string` | 否 | Cron 计划。 |

## 迁移

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `name` | `string` | 是 | 必须引用同时具有备份和恢复操作的 `data_protect.data[].name`。 |
| `enabled` | `bool` | 否 | 启用或禁用此项的迁移。 |
