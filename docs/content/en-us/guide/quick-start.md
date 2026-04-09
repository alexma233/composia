# Quick Start

This guide will help you get Composia up and running in minutes using pre-built container images.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2.0+

## Installation

### 1. Create a Working Directory

Prepare an empty directory with this layout:

```text
composia/
├── docker-compose.yaml
└── configs/
    └── config.compose.yaml
```

### 2. Download the Startup Files

Download these files from the repository and save them using the layout above:

- [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml)
- [`configs/config.compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/config.compose.yaml)

If you want to keep the default `secrets` section in `config.compose.yaml`, also download:

- [`configs/age-identity.key`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/age-identity.key)
- [`configs/age-recipients.txt`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/configs/age-recipients.txt)

### 3. Adjust the Platform Configuration

Before startup, review and update at least these values:

- `controller.cli_tokens[].token`: controller access token used by the Web UI
- `controller.nodes[].token` and `agent.token`: node authentication token, which must match on both sides
- `COMPOSIA_CLI_TOKEN` in `docker-compose.yaml`: it must match one enabled token under `controller.cli_tokens`

If you do not want to use the bundled age key files, replace the `secrets` configuration in `configs/config.compose.yaml` or remove that section before startup.

### 4. Start Composia

The following command starts Composia with the local `docker-compose.yaml` and `configs/config.compose.yaml` files:

```bash
docker compose up -d
```

This starts the following long-running services:

| Service | Port | Description |
|---------|------|-------------|
| controller | `:7001` | Control Plane API |
| web | `:3000` | Web Management Interface |
| agent | - | Execution Agent (connects to local Docker) |

The Compose file also runs a one-shot `init-repo-controller` container first to initialize the controller Git working tree volume.

### 5. Access the Interface

Open your browser and visit `http://localhost:3000`.

The Web UI does not prompt for a token. It uses the `COMPOSIA_CLI_TOKEN` environment variable injected into the web server process. That value must match one enabled token under `controller.cli_tokens`.

### 6. Deploy Your First Service

1. Navigate to the **Services** page in the web interface
2. Click **Create service**
3. Enter a service name
4. Add your `docker-compose.yaml` content in the editor
5. Define the target nodes in `composia-meta.yaml`
6. Click **Deploy**

### 7. Stop Composia

This stops the Composia stack started from the local `docker-compose.yaml`:

```bash
docker compose down
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
