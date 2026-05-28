---
title: "Service Configuration"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Each service lives in a directory inside the controller repository. A service directory contains `composia-meta.yaml` and one or more Docker Compose files.

Minimal service:

```yaml {filename="composia-meta.yaml"}
name: my-app
nodes:
  - main
```

With the default behavior, Composia looks for `docker-compose.yaml` in the same directory.

## Top-level keys

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Unique service name. |
| `project_name` | `string` | No | Docker Compose project name override. Defaults to a normalized service name. |
| `compose_files` | `[]string` | No | Compose file paths relative to the service directory. |
| `enabled` | `bool` | No | Whether the service is active. Defaults to `true`. |
| `nodes` | `[]string` | Yes | Target node IDs. Each must exist in `controller.nodes`. |
| `infra` | `object` | No | Declares this service as Caddy, Rustic, or config-only infrastructure. |
| `network` | `object` | No | Caddy and DNS settings. |
| `update` | `object` | No | Image update settings. |
| `data_protect` | `object` | No | Backup and restore data definitions. |
| `backup` | `object` | No | Scheduled backups for protected data. |
| `migrate` | `object` | No | Migration-enabled protected data. |
| `auto_deploy` | `bool` | No | Auto-deploy this service after repository changes. |

`compose_files` entries must be relative paths, must stay inside the service directory, and must not be duplicated.

## Infrastructure services

### `infra.caddy`

Declares the repository's Caddy infrastructure service.

```yaml
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

| Key | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Compose service name. Defaults to `caddy`. |
| `config_dir` | `string` | Caddy config directory. Defaults to `/etc/caddy`. |

Only one service can be declared as Caddy infrastructure.

### `infra.rustic`

Declares the repository's Rustic infrastructure service.

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

| Key | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Compose service name. Defaults to `rustic`. |
| `profile` | `string` | Rustic profile name. |
| `data_protect_dir` | `string` | Directory used by data protection workflows. |
| `init_args` | `[]string` | Extra args passed to `rustic init`. Empty entries are rejected. |

Only one service can be declared as Rustic infrastructure.

### `infra.config`

Declares a config-only infrastructure service.

```yaml
infra:
  config: {}
```

Config-only services cannot be combined with `infra.caddy` or `infra.rustic`. Their `data_protect` actions can only use `files.copy`.

## Network

### `network.caddy`

```yaml
network:
  caddy:
    enabled: true
    source: Caddyfile
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `enabled` | `bool` | No | Enables Caddy management. Defaults to `false`. |
| `source` | `string` | Cond. | Caddyfile path relative to the service directory. Required when enabled. |

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
    comment: Managed by Composia
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `provider` | `string` | Yes | `cloudflare`, `alidns`, `dnspod`, `route53`, or `huaweicloud`. |
| `hostname` | `string` | Yes | DNS hostname. |
| `record_type` | `string` | No | Empty, `A`, `AAAA`, or `CNAME`. |
| `value` | `string` | No | DNS record value. Multi-node services should set this explicitly. |
| `proxied` | `bool` | No | Provider-specific proxy toggle, currently relevant for Cloudflare. |
| `ttl` | `uint32` | No | DNS TTL. |
| `comment` | `string` | No | DNS record comment. |

## Image updates

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

| Key | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Enables update checks for this service. |
| `auto_apply` | `bool` | Apply detected updates automatically. |
| `check_schedule` | `string` | Cron schedule for update checks. |
| `backup_before_update` | `bool` | Run backups before applying updates. |
| `backup_data` | `[]object` | Protected data items to back up before update. |
| `digest_pin` | `bool` | Pin images by digest. |
| `discovery_sources` | `map[string]object` | Reusable discovery sources. Named sources cannot reference another source. |
| `images` | `map[string]object` | Per-image update definitions. |

### `update.backup_data[]`

| Key | Type | Description |
|-----|------|-------------|
| `name` | `string` | Protected data item name. |
| `enabled` | `bool` | Include or exclude this item. |

### `update.images.<name>`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `image` | `string` | Yes | Image repository. |
| `auto_apply` | `bool` | No | Per-image auto-apply override. |
| `check_schedule` | `string` | No | Per-image check schedule. |
| `backup_before_update` | `bool` | No | Per-image backup toggle. |
| `digest_pin` | `bool` | No | Per-image digest pin toggle. |
| `current` | `object` | Yes | Current version source. |
| `discovery` | `object` or `string` | Yes | Discovery config or named discovery source reference. |
| `filter` | `object` | Cond. | Required unless discovery is `digest`. |

### `current`

Specify exactly one of:

| Key | Description |
|-----|-------------|
| `tag` | Static current tag. |
| `env.file` + `env.key` | Read current tag from an env file. `file` must be relative and stay inside the service directory. |
| `yaml.file` + `yaml.path` | Read current tag from a YAML file. `file` must be relative and stay inside the service directory. |

### `discovery`

| Key | Type | Description |
|-----|------|-------------|
| `sources` | `[]object` | At least one source. |
| `combine` | `string` | Empty, `merge`, or `first_success`. |
| `include_prerelease` | `bool` | Include prerelease versions. |

Discovery source types:

| Type | Required keys | Notes |
|------|---------------|-------|
| `auto` | None | `repo_url` is optional and must be a valid URL if set. Must be the only source. |
| `probe` | None | Requires `semver` filter when a filter is present. |
| `registry` | None | Registry tag discovery. |
| `digest` | None | Must be the only source. `filter` must be omitted. |
| `github` | `repo` | `repo` is `owner/repo`. |
| `gitlab` | `project` | GitLab project ID or path. |
| `forgejo` | `repo` | `repo` is `owner/repo`. |

### `filter`

| Type | Required keys | Notes |
|------|---------------|-------|
| `semver` | None | `allow` may contain `patch`, `minor`, `major`. |
| `date` | `format` | Date format used to parse tags. |
| `regex` | `pattern`, `order` | `order` must be `numeric` or `lexicographic`. |
| `latest` | None | Uses the latest candidate. |

## Data protection

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

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Unique data item name. |
| `backup` | `object` | No | Backup action. |
| `restore` | `object` | No | Restore action. |

### Data action

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `strategy` | `string` | Yes | `files.copy`, `files.copy_after_stop`, `database.pgdumpall`, or `database.pgimport`. |
| `service` | `string` | Cond. | Required for `database.*` strategies. Compose service name. |
| `include` | `[]string` | Cond. | Required for `files.*` strategies. |

## Backups

```yaml
backup:
  data:
    - name: db
      provider: rustic
      enabled: true
      schedule: "0 2 * * *"
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Must reference a `data_protect.data[].name` with a backup action. |
| `provider` | `string` | No | Backup provider name. |
| `enabled` | `bool` | No | Enable or disable this backup entry. |
| `schedule` | `string` | No | Cron schedule. |

## Migration

```yaml
migrate:
  data:
    - name: db
      enabled: true
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `name` | `string` | Yes | Must reference a `data_protect.data[].name` with both backup and restore actions. |
| `enabled` | `bool` | No | Enable or disable migration for this item. |
