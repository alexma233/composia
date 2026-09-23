---
title: "Service Configuration"
date: '2026-05-26T00:00:00+08:00'
weight: 10
---

Each service lives in a top-level directory inside the controller repository. A service directory contains `composia-meta.yaml` and one or more Docker Compose files.

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

`compose_files` entries must be relative paths, must stay inside the service directory, and must not be duplicated. All file references in `composia-meta.yaml` use repository filenames. An encrypted reference therefore includes `.enc`; Composia removes that suffix after decryption when the agent accesses the runtime file. References inside Compose files, Caddyfiles, scripts, and other native formats use the runtime filename without `.enc`.

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
| `data_protect_dir` | `string` | Container path mapped to the agent's `{StateDir}/data-protect`. |
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
    - hostname: app.example.com
      record_type: A
      value: 203.0.113.10
    - hostname: app.example.com
      record_type: AAAA
      value: 2001:db8::10
```

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `provider` | `string` | No | `cloudflare`, `alidns`, `dnspod`, `route53`, or `huaweicloud`. It can be inferred from the configured zone. |
| `hostname` | `string` | Yes | DNS hostname. |
| `record_type` | `string` | No | Empty, `A`, `AAAA`, or `CNAME`. |
| `value` | `string` | No | DNS record value. Multi-node services should set this explicitly. |
| `proxied` | `bool` | No | Provider-specific proxy toggle, currently relevant for Cloudflare. |
| `ttl` | `uint32` | No | DNS TTL. |
| `comment` | `string` | No | DNS record comment. |

A service can define any number of DNS records. The same hostname may use different record types such as `A` and `AAAA`; duplicate hostname + record type entries and combining `CNAME` with other types are rejected.

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

### Image check prerequisites

Image checks are read-only: they never download or install a service bundle, pull images, or recreate containers. Before discovering updates, the agent:

1. Fetches a task-authorized manifest of all controller-managed persistent service files, including decrypted runtime files, and verifies their contents locally using SHA-256. Bundle permission limits and the executable bit are checked; a restrictive extraction umask is allowed. Missing or modified files require deployment before checking again. Unmanaged generated files and runtime data are not hashed, but an unmanaged `.env` is rejected because it changes Compose interpolation.
2. Compares `docker compose config --hash` with each applicable container's `com.docker.compose.config-hash`. A deployment outside Composia is accepted when the project, service, and configuration match; no successful Composia task record is required. Existing replicas must match the expected configuration hash; a stopped matching container is configuration-consistent. Inactive profile services without containers are skipped.
3. Inspects each running container's immutable image ID, rather than the image currently cached under its tag. Missing or ambiguous repository digests and inconsistent replicas fail the check instead of guessing an update baseline.

Configuration differences fail only the check task; they do not overwrite the service's runtime status or its update timestamp. A newer controller revision is allowed if the service's rendered files are unchanged. Checks observe a point in time, not a continuous guarantee against external changes.

Compose 5.5.1 or newer is required for the corrected `env_file` hash behavior. Agent images install `docker-cli-compose` from a tagged Alpine edge repository without moving the remaining packages to edge. The Compose hash does not include bind-mounted file contents or every deployment setting; it is not a complete runtime convergence check. Keep generated Caddy files outside replaceable service directories, preferably at the default `<state_dir>/caddy/generated`.

### Most recent consistency check

Consistency is an independent configuration snapshot per service and node, not a value inferred from the image-check task status. Initially, only `image_check` passively runs and reports it, before running-image observation and remote registry discovery. There is no separate consistency action, timer, automatic expiry, or history ledger.

Web instances and `composia service <service>` (including JSON output, without requiring `--containers`) show separate **Files** and **Compose** outcomes:

| Status | Meaning |
|--------|---------|
| `unknown` | Not checked, including a stage skipped because an earlier stage failed. |
| `consistent` | The checked configuration matched. |
| `drifted` | A definite file, permission, container, or configuration-hash mismatch was found. |
| `error` | The check could not be completed, for example due to access, transport, or tool failures. |
| `not_applicable` | Compose does not apply to an `infra.config` service. |

The most recent snapshot includes its server-recorded check time, target repository revision, source task, and stage-specific reasons. It remains historical evidence until the next check replaces it, even if runtime state or repository configuration changes afterward. Running state and image digest availability are separate observations: a stopped container can have consistent configuration while image observation fails. A later registry or image-observation error does not overwrite the recorded configuration outcomes.

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
        user: app
      restore:
        strategy: database.pgimport
        service: postgres
        user: app
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
| `user` | `string` | No | PostgreSQL role passed to `pg_dumpall` or `psql`. If omitted, Composia checks the resolved service environment for `PGUSER`, `POSTGRES_USER`, `POSTGRESQL_USERNAME`, then `POSTGRESQL_USER`, in that order. |
| `include` | `[]string` | Cond. | Required for `files.*` strategies. Use `./...` or paths containing `/` for service paths; bare names are Docker volume names. |

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
