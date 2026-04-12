# Quick Start

This guide will help you get Composia up and running in minutes using pre-built container images.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2.0+

## Installation

### 1. Create a Working Directory

Create a local working directory and download the production Compose file directly instead of cloning the full repository:

```bash
mkdir -p composia/config
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml -o composia/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o composia/.env
cd composia
```

The directory layout looks like this:

```text
composia/
├── docker-compose.yaml
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

### 2. Download the Startup Files

The published `docker-compose.yaml` is production-ready. Use the [Configuration Guide](./configuration), [Controller Configuration](./configuration/controller), and [Agent Configuration](./configuration/agent) to write `config/config.yaml`, then update the placeholder values in `.env` as needed.

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
- `COMPOSIA_ACCESS_TOKEN` in `.env`: it must match one enabled token under `controller.access_tokens`
- `COMPOSIA_CONFIG_DIR`, `COMPOSIA_CONTROLLER_REPO_DIR`, `COMPOSIA_CONTROLLER_STATE_DIR`, `COMPOSIA_CONTROLLER_LOG_DIR`, `COMPOSIA_AGENT_REPO_DIR`, and `COMPOSIA_AGENT_STATE_DIR` in `.env`: host-side bind mount paths used by Compose
- `DOCKER_SOCK_GID` in `.env`: the GID of the host's `/var/run/docker.sock`; the agent must join this group to access the local Docker daemon
- `WEB_LOGIN_USERNAME` in `.env`: local username for the Web login page
- `WEB_LOGIN_PASSWORD_HASH` in `.env`: Argon2 password hash for the Web login page
- `WEB_SESSION_SECRET` in `.env`: random secret used to sign the Web session cookie
- `ORIGIN` in `.env`: set this to the exact address you use to open the Web UI, such as `http://localhost:3000`, `http://127.0.0.1:3000`, or your production domain. Do not mix hosts, or form login may fail with `Cross-site POST form submissions are forbidden`

First, look up the Docker socket GID on the host and write it into `.env`:

```bash
ls -ln /var/run/docker.sock
```

For example, if the output is:

```text
srw-rw---- 1 0 131 0 Mar  5 01:52 /var/run/docker.sock
```

set this in `.env`:

```env
DOCKER_SOCK_GID=131
```

If this value is wrong, the `agent` will fail with `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`.

The default `docker-compose.yaml` reads its bind mount paths from `.env`. With the default values, the controller repo, state, and log directories live under `./data/`, the agent state directory lives at `./data/state-agent`, and only the agent repo uses the fixed host absolute path `/data/repo-agent`. Create these directories before startup:

```bash
mkdir -p ./data/repo-controller ./data/state-controller ./data/logs ./data/state-agent
sudo mkdir -p /data/repo-agent
sudo chown 65532:65532 /data/repo-agent
```

If you change these paths, update both `.env` and `config/config.yaml`. `agent.repo_dir`, the host-side mount paths, and the container-side mount paths must match exactly, or bind mounts in managed service Compose files may resolve to the wrong host location and file mounts can fail.

Generate the Argon2 hash before startup. You can generate it directly in this page:

<ClientOnly>
  <Argon2Generator />
</ClientOnly>

If you prefer the CLI instead:

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password 'replace-with-your-password'
```

Generate a session secret with a long random value, for example:

```bash
openssl rand -hex 32
```

If you do not want to use `secrets` yet, remove the `secrets` section from `config/config.yaml` before startup.

### 4. Start Composia

After updating the placeholder values in `.env`, run this command from the working directory to start Composia with `docker-compose.yaml`, `.env`, and `config/config.yaml`:

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

Make sure `.env` uses an `ORIGIN` value that exactly matches the address you will open in the browser, then visit it, for example `http://localhost:3000`.

If you access the Web UI through an SSH tunnel, local port forwarding, or a reverse proxy, update `ORIGIN` to match that address. For example:

- If you open `http://localhost:3000`, set `ORIGIN=http://localhost:3000`
- If you open `http://127.0.0.1:3000`, set `ORIGIN=http://127.0.0.1:3000`
- If you open `https://composia.example.com`, set `ORIGIN=https://composia.example.com`

`localhost` and `127.0.0.1` are different origins and are not interchangeable.

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

This stops the Composia stack started from `docker-compose.yaml` in your working directory:

```bash
docker compose down
```

## Image Registry Options

By default, `docker-compose.yaml` uses the self-hosted Forgejo registry. To use GitHub Container Registry instead, replace the image references:

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
