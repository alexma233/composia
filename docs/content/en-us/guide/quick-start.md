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

### 2. Review the Bundled Configuration

The repository already includes `configs/config.compose.yaml`.

- Use it as-is for a local quick start.
- Edit it only if you need different tokens or paths.
- Keep the bundled `configs/age-identity.key` and `configs/age-recipients.txt` files if you keep the default secrets section.

### 3. Start the Stack

The following command uses the repository root `docker-compose.yaml`. The file you created above, `configs/config.compose.yaml`, is consumed by that Compose stack as the platform config.

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

### 4. Access the Interface

Open your browser and visit `http://localhost:3000`.

The Web UI does not prompt for a token. It uses the `COMPOSIA_CLI_TOKEN` environment variable injected into the web server process. In the provided `docker-compose.yaml`, that value is set to `dev-admin-token`.

### 5. Deploy Your First Service

1. Navigate to the **Services** page in the web interface
2. Click **Create service**
3. Enter a service name
4. Add your `docker-compose.yaml` content in the editor
5. Define the target nodes in `composia-meta.yaml`
6. Click **Deploy**

### 6. Stop the Stack

This stops the container stack started from the repository root `docker-compose.yaml`:

```bash
docker compose down
```

To remove the Compose volumes as well, add the `-v` flag:

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
