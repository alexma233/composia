---
title: "Image Updates"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia detects new image tags and can apply updates automatically. Image check tasks run on the agent and report findings to the controller.

## How it works

The controller schedules periodic `image_check` tasks according to the service's update configuration. Each check:

1. The agent downloads the service bundle.
2. Reads `docker compose config --format json` to discover running images.
3. Reports local and remote digests for each image.
4. For images configured in `update.images`, checks for new candidate tags using the configured discovery sources.
5. Reports results to the controller. The controller records available updates and can auto-apply them.

## Controller defaults

Global defaults are set in the controller config:

```yaml
controller:
  updates:
    default_check_schedule: "0 */6 * * *"
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
        token: "REPLACE"
        api_url: "https://api.github.com"
```

The service-level `update` section overrides these defaults.

## Service configuration

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
    upstream-gh:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    api:
      image: ghcr.io/example/api
      current:
        env:
          file: .env
          key: API_VERSION
      discovery: upstream-gh
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update` top-level

| Key | Type | Description |
|-----|------|-------------|
| `enabled` | `bool` | Enables update checks for this service. |
| `auto_apply` | `bool` | Apply detected updates automatically. |
| `check_schedule` | `string` | Cron schedule for update checks. |
| `backup_before_update` | `bool` | Run a backup before applying an update. |
| `backup_data` | `[]object` | Protected data items to back up before updating. Each item has a `name` and optional `enabled`. |
| `digest_pin` | `bool` | Pin images by digest for reproducibility. |
| `discovery_sources` | `map[string]object` | Named reusable discovery configurations. |
| `images` | `map[string]object` | Per-image update configuration. Keys are arbitrary names matching images to check. |

### `images.<name>`

| Key | Type | Required | Description |
|-----|------|----------|-------------|
| `image` | `string` | Yes | Full image reference, for example `ghcr.io/example/api`. |
| `auto_apply` | `bool` | No | Per-image auto-apply override. |
| `check_schedule` | `string` | No | Per-image check schedule. |
| `backup_before_update` | `bool` | No | Per-image backup toggle. |
| `digest_pin` | `bool` | No | Per-image digest pin toggle. |
| `current` | `object` | Yes | How to find the currently deployed version. |
| `discovery` | `object` or `string` | Yes | Discovery configuration or reference to a named `discovery_sources` entry. |
| `filter` | `object` | Cond. | Version filter. Required unless discovery mode is `digest`. |

### `current`

Exactly one of these sources must be specified:

**Static tag:**

```yaml
current:
  tag: "v1.2.3"
```

**Environment file:**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

The `file` path is relative to the service directory. Composia reads the file, looks for `KEY=VALUE` lines, and extracts the value.

**YAML file:**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

The `path` is a dot-separated path into the YAML document tree. The value at that path must be a scalar.

### Discovery

Discovery sources can be:

**Named reference** to a `discovery_sources` entry:

```yaml
discovery: upstream-gh
```

**Inline definition:**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

Discovery source types:

| Type | Required keys | Behavior |
|------|---------------|----------|
| `probe` | None | Semver probing: searches for higher versions by probing registry manifests. Requires a `semver` filter. |
| `registry` | None | Lists all tags from the image registry. |
| `auto` | None (optional `repo_url`) | Tries `probe` then `registry` as a merged discovery. Must be the only source in a discovery config. |
| `digest` | None | Compares remote digest against local digest only. No tag comparison. `filter` must be omitted. Must be the only source. |
| `github` | `repo` (`owner/repo`) | Queries GitHub releases. Processed on the controller side. |
| `gitlab` | `project` | Queries GitLab releases. Processed on the controller side. |
| `forgejo` | `repo` (`owner/repo`) | Queries Forgejo releases. Processed on the controller side. |

`combine` accepts `merge` (union of all source results) or `first_success` (first source that returns results wins).

`include_prerelease` includes pre-release versions in GitHub, GitLab, and Forgejo release queries.

### Filter

| Type | Required keys | Behavior |
|------|---------------|----------|
| `semver` | None | Filter by semantic version. `allow` may contain `patch`, `minor`, `major`. |
| `date` | `format` | Parse tags as dates using the given format. |
| `regex` | `pattern`, `order` | Filter by regex. Order must be `numeric` or `lexicographic`. |
| `latest` | None | Take the latest tag without filtering. |

#### Semver probing

With `type: probe` and a `semver` filter, Composia searches for candidate tags by constructing version numbers and checking if the corresponding registry manifest exists. It probes patch, minor, and major bumps according to the `allow` list, using exponential search with binary refinement to find the highest available version.

## Digest mode

When all discovery sources in a configuration have `type: digest`, no tag comparison is performed. Composia only compares the remote image digest against the local digest:

```yaml
discovery:
  sources:
    - type: digest
```

When `digest` is set as the discovery mode, `filter` must be omitted. If a digest differs, an update is considered available.

## Image observations

During deploy and update tasks, the agent also collects image observations for all compose services. These include local and remote digests, reported to the controller regardless of whether `update.images` is configured. This provides image state visibility in the web UI and CLI.
