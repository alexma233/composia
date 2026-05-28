---
title: "Reverse Proxy"
date: '2026-05-26T00:00:00+08:00'
weight: 30
---

Composia integrates with Caddy for reverse proxy management. The Caddy infrastructure service runs as a normal Docker Compose service, and Composia syncs Caddy configuration files on deploy and stop.

## Architecture

```
Controller repo
  ├── caddy/
  │   ├── docker-compose.yaml   (Caddy Compose service)
  │   ├── Caddyfile             (main Caddy config, imports generated files)
  │   └── composia-meta.yaml    (declares infra.caddy)
  ├── my-app/
  │   ├── docker-compose.yaml
  │   ├── Caddyfile             (service-specific Caddy config)
  │   └── composia-meta.yaml    (declares network.caddy)
  └── ...
```

At deploy time, Composia copies each service's Caddyfile into a generated directory and then triggers a Caddy reload.

## Infrastructure setup

Declare exactly one Caddy infrastructure service in the repo:

```yaml {filename="caddy/composia-meta.yaml"}
name: caddy
nodes:
  - main
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
```

The main Caddyfile in the Caddy service directory should import the generated files:

```caddy {filename="caddy/Caddyfile"}
import /etc/caddy/generated/*.caddy
```

| Key | Type | Description |
|-----|------|-------------|
| `compose_service` | `string` | Compose service name. Defaults to `caddy`. |
| `config_dir` | `string` | Caddy config directory inside the container. Defaults to `/etc/caddy`. |

Only one service in the repository can be declared as Caddy infrastructure.

## Service configuration

For each service that needs a reverse proxy entry, enable Caddy in `composia-meta.yaml` and provide a Caddyfile:

```yaml {filename="my-app/composia-meta.yaml"}
name: my-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: Caddyfile
```

The `source` path is relative to the service directory and must stay inside it. The file can be named anything, but `Caddyfile` is the convention.

```caddy {filename="my-app/Caddyfile"}
app.example.com {
    reverse_proxy app:8080
}
```

## How sync works

During a deploy or update task, the agent runs a Caddy sync step after `compose up`:

1. Read `network.caddy.source` from the service's `composia-meta.yaml`.
2. Copy the source file to `<agent_state_dir>/caddy/generated/<service_dir>.caddy`.
3. Run `docker compose exec <caddy_service> caddy reload --config <Caddyfile> --adapter caddyfile`.

The generated file name is derived from the service directory name. For `my-app`, the file is `my-app.caddy`.

During a stop task, the generated Caddy file is removed.

## Caddy sync task

A standalone `caddy_sync` task rebuilds Caddy configuration without deploying services. It can operate in two modes:

**Full rebuild** (`full_rebuild: true`): deletes all generated `.caddy` files from the generated directory, then re-syncs all Caddy-managed services.

**Targeted sync**: syncs only the specified service directories.

Trigger through the web UI or CLI:

```bash
composia service caddy-sync my-app
```

## Caddy reload task

A `caddy_reload` task runs `caddy reload` inside the Caddy container without changing any files. Use it after manually editing the main Caddyfile:

```bash
composia node reload-caddy main
```

## Agent configuration

The agent config has an optional Caddy section:

```yaml
agent:
  caddy:
    generated_dir: "/data/state-agent/caddy/generated"
```

| Key | Type | Description |
|-----|------|-------------|
| `generated_dir` | `string` | Generated Caddy config directory. Defaults to `<state_dir>/caddy/generated`. |

The generated directory must be inside a path that the Caddy container can read. The Caddy compose service must have a volume mounting this directory to the path imported in the main Caddyfile.
