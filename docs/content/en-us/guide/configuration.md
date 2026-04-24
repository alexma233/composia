# Configuration Guide

This page now keeps the overview only. Platform configuration has been split into dedicated documents by feature so each topic is easier to scan.

Configuration loading is strict. Unknown fields are rejected during startup.

## Configuration Types

| Configuration Type | File | Scope | Description |
|-------------------|------|-------|-------------|
| Platform Config | `config/config.yaml` | Entire platform | Defines how Controller and Agents start |
| Service Config | `composia-meta.yaml` | Individual service | Defines service deployment targets and features |

For service-side configuration, see [Service Definition](./service-definition).

## Platform Configuration Overview

### Full Configuration Example

```yaml
controller:
  # Network configuration
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"

  # Directory configuration
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"

  # Authentication configuration
  access_tokens:
    - name: "compose-admin"
      token: "replace-this-token"
      enabled: true
      comment: "Primary admin token"

  # Node configuration
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
      public_ipv6: "2001:db8::10"
    - id: "edge"
      display_name: "Edge"
      enabled: true
      token: "edge-agent-token"

  # Git sync configuration (optional)
  git:
    remote_url: "https://git.example.com/infra/composia.git"
    branch: "main"
    pull_interval: "30s"
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: "git"
      token_file: "/app/configs/git-token.txt"

  # DNS configuration (optional)
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"

  # Backup configuration (optional)
  backup:
    default_schedule: "0 2 * * *"

  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "15 3 * * *"
      prune_schedule: "45 3 * * *"

  # Secrets configuration (optional)
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true

agent:
  controller_addr: "http://controller:7001"
  controller_grpc: false
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

## Read By Feature

- [Controller Configuration](./configuration/controller) — base fields, access tokens, node definitions, and a minimal single-node example
- [Agent Configuration](./configuration/agent) — required fields, same-file constraints, and Caddy output directory
- [Git Remote Sync](./configuration/git-sync) — `controller.git` fields and example
- [DNS Configuration](./configuration/dns) — `controller.dns` and how it relates to service-side DNS settings
- [Backup Configuration](./configuration/backup) — `controller.backup`, `controller.rustic`, and service-side schedule override rules
- [Secrets Configuration](./configuration/secrets) — `controller.secrets`, age key files, and enablement requirements
- [Configuration Security](./configuration/security) — token and key file handling recommendations
- [Configuration Verification](./configuration/verification) — validating config from local source-based development

## Related Documentation

- [Service Definition](./service-definition) — service-side `composia-meta.yaml` configuration
- [DNS Configuration](./dns) — service-side DNS rules, auto-derived values, and record updates
- [Caddy Configuration](./caddy) — Caddy infrastructure, configuration fragments, and automated sync
- [Backup & Migration](./backup-migrate) — rustic infrastructure and data protection workflow
- [Quick Start](./quick-start) — fast setup with containers
