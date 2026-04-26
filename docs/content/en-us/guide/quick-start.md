# Quick Start

Get Composia running and deploy your first service in minutes. For a complete walkthrough of every configuration option, see [Installation](./installation).

## Prerequisites

- Docker Engine 20.10+ and Docker Compose v2.0+

## Setup

```bash
mkdir -p composia/config
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml -o composia/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o composia/.env
cd composia
```

## Minimal Configuration

Edit `.env` and fill in at least these values:

```env
# Required
WEB_CONTROLLER_ADDR=http://controller:7001
WEB_BROWSER_CONTROLLER_ADDR=http://localhost:7001
WEB_CONTROLLER_ACCESS_TOKEN=replace-with-a-secure-token
WEB_LOGIN_USERNAME=admin
WEB_LOGIN_PASSWORD_HASH=<generate below>
WEB_SESSION_SECRET=<generate below>
ORIGIN=http://localhost:3000
DOCKER_SOCK_GID=<from ls -ln /var/run/docker.sock>
```

Generate the password hash and session secret:

```bash
docker run --rm authelia/authelia:latest authelia crypto hash generate argon2 --password 'your-password'
openssl rand -hex 32
```

Create a minimal `config/config.yaml`:

```yaml
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "admin"
      token: "replace-with-a-secure-token"
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
```

Create the required directories:

```bash
mkdir -p ./data/repo-controller ./data/state-controller ./data/logs ./data/state-agent
sudo mkdir -p /data/repo-agent && sudo chown 65532:65532 /data/repo-agent
```

## Start

```bash
docker compose up -d
```

Open `http://localhost:3000` and sign in with your configured credentials.

## Deploy Your First Service

1. Navigate to **Services** > **Create service**
2. Enter a service name
3. Paste your `docker-compose.yaml` content in the editor
4. Set the target node to `main` in `composia-meta.yaml`
5. Click **Deploy**

Here is a minimal example to get started:

```yaml
# composia-meta.yaml
name: hello
nodes:
  - main
```
```yaml
# docker-compose.yaml
services:
  hello:
    image: nginx:alpine
    ports:
      - "8080:80"
```

## Next Steps

- [Installation](./installation) — full setup walkthrough with all options
- [Core Concepts](./core-concepts) — understand Services, Instances, Containers, and Nodes
- [Architecture](./architecture) — system architecture and data flow
- [Configuration Guide](./configuration) — all platform configuration options
- [Service Definition](./service-definition) — create and configure services
