# Quick Start

This guide will help you get Composia up and running in minutes using pre-built container images.

## Prerequisites

- Docker Engine + Docker Compose v2

## Installation

### 1. Create configuration file

Create the configuration file for the container stack:

```bash
mkdir -p configs
cat > configs/config.compose.yaml << 'EOF'
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  cli_tokens:
    - name: "compose-admin"
      token: "dev-admin-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
  rustic:
    main_nodes:
      - "main"
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
EOF
```

### 2. Start the stack

```bash
docker compose up -d
```

By default, `docker-compose.yaml` uses the self-hosted Forgejo registry.
If you prefer GHCR, replace the image references in `docker-compose.yaml` with:

```yaml
ghcr.io/alexma233/composia:latest
ghcr.io/alexma233/composia-web:latest
```

This will pull the pre-built images and start:
- `controller` on `:7001`
- `web` on `:3000`
- `agent` connected to the local Docker socket

### 3. Access the interface

Open your browser and visit `http://localhost:3000` to view the web interface.

The default development CLI token is `dev-admin-token`.

Published images:
- Default: `forgejo.alexma.top/alexma233/composia` and `forgejo.alexma.top/alexma233/composia-web`
- Alternative: `ghcr.io/alexma233/composia` and `ghcr.io/alexma233/composia-web`

### 4. Stop the stack

```bash
docker compose down
```

## Next Steps

- Learn about the [Architecture](./architecture)
- Check the API documentation
- Deploy your first service

## Development

For local development with source code, see the [Development Guide](./development).
