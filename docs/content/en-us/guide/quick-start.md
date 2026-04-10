# Quick Start

This guide will help you get Composia up and running in minutes using pre-built container images.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2.0+

## Installation

### 1. Create a Working Directory

Prepare an empty directory and create the startup files yourself:

```text
composia/
├── docker-compose.yaml
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

### 2. Download the Startup Files

Write `docker-compose.yaml` and `config/config.yaml` yourself using the [Configuration Guide](./configuration), [Controller Configuration](./configuration/controller), and [Agent Configuration](./configuration/agent).

If you enable `secrets`, follow [Secrets Configuration](./configuration/secrets) and generate your own age key pair:

```bash
mkdir -p config
age-keygen -o config/age-identity.key
grep "public key:" config/age-identity.key | awk '{print $4}' > config/age-recipients.txt
```

### 3. Adjust the Platform Configuration

Before startup, review and update at least these values:

- `controller.access_tokens[].token`: controller access token used by the Web UI
- `controller.nodes[].token` and `agent.token`: node authentication token, which must match on both sides
- `COMPOSIA_ACCESS_TOKEN` in `docker-compose.yaml`: it must match one enabled token under `controller.access_tokens`
- `WEB_LOGIN_USERNAME` in `docker-compose.yaml`: local username for the Web login page
- `WEB_LOGIN_PASSWORD_HASH` in `docker-compose.yaml`: Argon2 password hash for the Web login page
- `WEB_SESSION_SECRET` in `docker-compose.yaml`: random secret used to sign the Web session cookie

Generate the Argon2 hash before startup:

```bash
cd web
bun -e "import { hash } from 'argon2'; console.log(await hash(Bun.argv[2]));" -- "replace-with-your-password"
```

Generate a session secret with a long random value, for example:

```bash
openssl rand -hex 32
```

If you do not want to use `secrets` yet, remove the `secrets` section from `config/config.yaml` before startup.

### 4. Start Composia

The following command starts Composia with the local `docker-compose.yaml` and `config/config.yaml` files:

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

The Web UI uses two auth layers:

- The browser signs in with `WEB_LOGIN_USERNAME` and the password represented by `WEB_LOGIN_PASSWORD_HASH`.
- The web server uses `COMPOSIA_ACCESS_TOKEN` to call the controller.

The browser does not receive `COMPOSIA_ACCESS_TOKEN`. After login it only stores a signed HttpOnly session cookie.

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
- [Configuration Guide](./configuration) — View the platform configuration overview
- [Controller Configuration](./configuration/controller) — Learn the base fields, tokens, and node configuration
- [Agent Configuration](./configuration/agent) — Learn the agent requirements and Caddy output directory
- [Architecture](./architecture) — Understand how the system works

## Local Development

For development with source code, see the [Development Guide](./development).
