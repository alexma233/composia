# Quick Start

This guide will help you get Composia up and running in minutes using pre-built container images.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2.0+

## Installation

### 1. Clone the Repository

```bash
git clone https://forgejo.alexma.top/alexma233/composia.git
cd composia
```

### 2. Create Configuration

Create the platform configuration file:

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

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
EOF
```

### 3. Start the Stack

```bash
docker compose up -d
```

This will start the following services:

| Service | Port | Description |
|---------|------|-------------|
| controller | `:7001` | Control Plane API |
| web | `:3000` | Web Management Interface |
| agent | - | Execution Agent (connects to local Docker) |

### 4. Access the Interface

Open your browser and visit `http://localhost:3000`. Log in with the default token:

- **CLI Token**: `dev-admin-token`

### 5. Deploy Your First Service

1. Navigate to the **Services** page in the web interface
2. Click **New Service**
3. Enter a service name and select the target node
4. Add your `docker-compose.yaml` content in the editor
5. Click **Deploy**

### 6. Stop the Stack

```bash
docker compose down
```

To remove data volumes as well, add the `-v` flag:

```bash
docker compose down -v
```

## Image Registry Options

By default, the `docker-compose.yaml` uses the self-hosted Forgejo registry. To use GitHub Container Registry instead, replace the image references:

```yaml
services:
  controller:
    image: ghcr.io/alexma233/composia:latest
  
  web:
    image: ghcr.io/alexma233/composia-web:latest
  
  agent:
    image: ghcr.io/alexma233/composia:latest
```

## Next Steps

- [Core Concepts](./core-concepts) — Understand the relationship between Services, Instances, Containers, and Nodes
- [Configuration Guide](./configuration) — Learn how to configure the controller and agents
- [Architecture](./architecture) — Understand how the system works

## Local Development

For development with source code, see the [Development Guide](./development).
