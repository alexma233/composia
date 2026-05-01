# Installation

This guide covers the full setup of Composia with pre-built container images. For a minimal path to try it out, see [Quick Start](./quick-start).

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2.0+

## Create a Working Directory

Download the production Compose file directly instead of cloning the full repository:

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
├── .env
└── config/
    ├── config.yaml
    ├── age-identity.key
    └── age-recipients.txt
```

## Platform Configuration

Use the [Configuration Guide](./configuration) together with the dedicated sub-pages below to write `config/config.yaml`:

- [Controller Configuration](./configuration/controller) — base fields, access tokens, and node definitions
- [Agent Configuration](./configuration/agent) — required fields and Caddy output directory
- [Git Remote Sync](./configuration/git-sync) — `controller.git` fields (optional)
- [DNS Configuration](./configuration/dns) — `controller.dns` and Cloudflare setup (optional)
- [Backup Configuration](./configuration/backup) — `controller.backup` and `controller.rustic` (optional)
- [Secrets Configuration](./configuration/secrets) — age-based encryption setup (optional)

If you enable `secrets`, generate your own age key pair:

```bash
age-keygen -o config/age-identity.key
grep "public key:" config/age-identity.key | awk '{print $4}' > config/age-recipients.txt
```

## Environment Variables

Review and update at least these values in `.env` before startup.

### Required

| Variable | Description |
|----------|-------------|
| `WEB_CONTROLLER_ACCESS_TOKEN` | Must match one token under `controller.access_tokens`; used by the web server to call the controller |
| `WEB_CONTROLLER_ADDR` | Controller base URL inside the Compose network |
| `WEB_BROWSER_CONTROLLER_ADDR` | Controller base URL reachable from the browser (for WebSocket terminal sessions) |
| `WEB_LOGIN_USERNAME` | Username for the Web login page |
| `WEB_LOGIN_PASSWORD_HASH` | Argon2 hash of the login password |
| `WEB_SESSION_SECRET` | Random secret for signing the session cookie |
| `ORIGIN` | Exact address used to open the Web UI (e.g. `http://localhost:3000`). Do not mix hosts, or form login may fail with `Cross-site POST form submissions are forbidden` |

### Directory Mounts

| Variable | Description |
|----------|-------------|
| `COMPOSIA_CONFIG_DIR` | Path to the `config/` directory on the host |
| `COMPOSIA_CONTROLLER_REPO_DIR` | Host path for the controller Git working tree |
| `COMPOSIA_CONTROLLER_STATE_DIR` | Host path for controller state (SQLite, caches) |
| `COMPOSIA_CONTROLLER_LOG_DIR` | Host path for task logs |
| `COMPOSIA_AGENT_REPO_DIR` | Host path for agent repo directory |
| `COMPOSIA_AGENT_STATE_DIR` | Host path for agent state directory |

### Docker Socket

| Variable | Description |
|----------|-------------|
| `DOCKER_SOCK_GID` | GID of the host's `/var/run/docker.sock`. The agent must join this group to access the local Docker daemon. |

Find it with:

```bash
ls -ln /var/run/docker.sock
```

If the output shows `srw-rw---- 1 0 131 ...`, set `DOCKER_SOCK_GID=131`. If this value is wrong, the agent will fail with `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`.

## Create Directories

With the default `.env` values, create these directories before startup:

```bash
mkdir -p ./data/repo-controller ./data/state-controller ./data/logs ./data/state-agent
sudo mkdir -p /data/repo-agent
sudo chown 65532:65532 /data/repo-agent
```

If you change these paths, update both `.env` and `config/config.yaml`. `agent.repo_dir`, the host-side mount paths, and the container-side mount paths must match exactly.

## Generate the Password Hash

Use the generator on this page:

<ClientOnly>
  <Argon2Generator />
</ClientOnly>

Or the CLI:

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password 'replace-with-your-password'
```

Generate a session secret:

```bash
openssl rand -hex 32
```

## ORIGIN Configuration

Set `ORIGIN` to the exact address you will open in the browser:

| Access method | `ORIGIN` value |
|---------------|----------------|
| Direct local access | `http://localhost:3000` |
| Explicit loopback | `http://127.0.0.1:3000` |
| Production domain | `https://composia.example.com` |

`localhost` and `127.0.0.1` are different origins and are not interchangeable. If you access the Web UI through an SSH tunnel or reverse proxy, update `ORIGIN` to match that address.

## Start Composia

```bash
docker compose up -d
```

This starts the following services:

| Service | Port | Description |
|---------|------|-------------|
| controller | `:7001` | Control Plane API |
| web | `:3000` | Web Management Interface |
| agent | — | Execution Agent (connects to local Docker) |

The Compose file also runs several init containers first (`init-repo-controller`, `init-perms-controller`, `init-config-perms`, `init-perms-agent`) to initialize the working tree volume and set up correct file permissions.

## Access the Interface

Open `http://localhost:3000` (or the address matching your `ORIGIN` value).

The Web UI uses two auth layers:
- The browser signs in with `WEB_LOGIN_USERNAME` and the password represented by `WEB_LOGIN_PASSWORD_HASH`.
- The web server uses `WEB_CONTROLLER_ACCESS_TOKEN` to call the controller. The browser never receives this token — after login it only stores a signed HttpOnly session cookie.

## Image Registry Options

By default, `docker-compose.yaml` uses the self-hosted Forgejo registry. Images are also published to GitHub Container Registry and Docker Hub.

### Forgejo (default)

```yaml
services:
  controller:
    image: forgejo.alexma.top/alexma233/composia-controller:latest
  web:
    image: forgejo.alexma.top/alexma233/composia-web:latest
  agent:
    image: forgejo.alexma.top/alexma233/composia-agent:latest
```

### GitHub Container Registry

```yaml
services:
  controller:
    image: ghcr.io/alexma233/composia-controller:latest
  web:
    image: ghcr.io/alexma233/composia-web:latest
  agent:
    image: ghcr.io/alexma233/composia-agent:latest
```

### Docker Hub

```yaml
services:
  controller:
    image: alexma233/composia-controller:latest
  web:
    image: alexma233/composia-web:latest
  agent:
    image: alexma233/composia-agent:latest
```

## Stop Composia

```bash
docker compose down
```

## Next Steps

- [Quick Start](./quick-start) — deploy your first service in 5 minutes
- [Core Concepts](./core-concepts) — understand Services, Instances, Containers, and Nodes
- [Architecture](./architecture) — understand how the system works
