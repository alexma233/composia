---
title: "Docker Compose"
date: '2026-05-26T00:00:00+08:00'
weight: 20
---

The Docker Compose stack runs the controller, one local agent, and the web UI from the canonical [`docker-compose.yaml`](https://forgejo.alexma.top/alexma233/composia/src/branch/main/docker-compose.yaml).

## Download the files

You do not need to clone the whole repository for a Docker Compose install. Download the compose file and environment template:

```bash
curl -LO https://forgejo.alexma.top/alexma233/composia/raw/branch/main/docker-compose.yaml
curl -L https://forgejo.alexma.top/alexma233/composia/raw/branch/main/.env.example -o .env
```

Edit `.env` before starting the stack. The template is grouped by role; for the all-in-one stack, keep all groups. See [Configuration](../configuration/) for the meaning of each variable.

Find the Docker socket group ID on the host:

```bash
stat -c '%g' /var/run/docker.sock
```

Set `DOCKER_SOCK_GID` to that value.

## Agent repository path

`COMPOSIA_AGENT_REPO_DIR` is mounted as:

```yaml
- ${COMPOSIA_AGENT_REPO_DIR}:${COMPOSIA_AGENT_REPO_DIR}
```

The host path and container path must be the same. The agent invokes the host Docker daemon, and the host Docker daemon resolves bind mounts from the host filesystem. If the service repository is mounted at a different path inside the agent container, Docker Compose can generate host paths that do not exist.

Use the same absolute path on both sides, for example:

```bash
COMPOSIA_AGENT_REPO_DIR=/data/repo-agent
```

Set `agent.repo_dir` in `config.yaml` to the same absolute path.

## Basic `config.yaml`

Create `config.yaml` inside `COMPOSIA_CONFIG_DIR`. The Docker Compose file mounts this directory at `/app/configs`.

```yaml {filename="config.yaml"}
controller:
  listen_addr: ":7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "web"
      token: "REPLACE_WITH_WEB_ACCESS_TOKEN"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "REPLACE_WITH_MAIN_AGENT_TOKEN"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

Set `WEB_CONTROLLER_ACCESS_TOKEN` in `.env` to the same value as `controller.access_tokens[0].token`.

## Web password

`WEB_LOGIN_PASSWORD_HASH` must be an Argon2id PHC hash. Generate it from a hidden prompt so the plaintext password is not written to shell history:

```bash
read -r -s -p 'Web password: ' COMPOSIA_WEB_PASSWORD; echo
printf '%s' "$COMPOSIA_WEB_PASSWORD" | docker run --rm -i -e NODE_NO_WARNINGS=1 node:24-alpine node -e 'const {randomBytes}=require("node:crypto");let p="";process.stdin.setEncoding("utf8");process.stdin.on("data",c=>p+=c);process.stdin.on("end",async()=>{const salt=randomBytes(16);const key=await crypto.subtle.importKey("raw-secret",Buffer.from(p),"Argon2id",false,["deriveBits"]);const bits=await crypto.subtle.deriveBits({name:"Argon2id",memory:65536,passes:3,parallelism:1,nonce:salt},key,256);const b64=b=>Buffer.from(b).toString("base64").replace(/=+$/g,"");console.log(`$argon2id$v=19$m=65536,t=3,p=1$${b64(salt)}$${b64(bits)}`);})'
unset COMPOSIA_WEB_PASSWORD
```

Paste the full `$argon2id$...` output into `.env`. The command uses Docker to run Node.js 24, so it does not require a local Node.js install.

Generate `WEB_SESSION_SECRET` with any cryptographically secure random generator, for example:

```bash
openssl rand -hex 32
```

## Start

```bash
docker compose up -d
docker compose ps
```

Open the web UI at `http://localhost:3000`.

## Role split

The compose file is sectioned by role:

- **Controller stack**: `init-repo-controller`, `init-perms-controller`, `controller`.
- **Web UI**: `web`.
- **Shared init**: `init-config-perms`.
- **Agent stack**: `init-perms-agent`, `agent`.

For anything beyond the all-in-one deployment, split those sections explicitly for your topology. Controller and web can run together or separately. Each agent node keeps the agent stack and its own Docker socket access.

## Images

Release images are published to Forgejo, GHCR, and Docker Hub:

| Component | Forgejo | GHCR | Docker Hub |
|-----------|---------|------|------------|
| CLI | `forgejo.alexma.top/alexma233/composia-cli` | `ghcr.io/alexma233/composia-cli` | `alexma233/composia-cli` |
| Controller | `forgejo.alexma.top/alexma233/composia-controller` | `ghcr.io/alexma233/composia-controller` | `alexma233/composia-controller` |
| Agent | `forgejo.alexma.top/alexma233/composia-agent` | `ghcr.io/alexma233/composia-agent` | `alexma233/composia-agent` |
| Web | `forgejo.alexma.top/alexma233/composia-web` | `ghcr.io/alexma233/composia-web` | `alexma233/composia-web` |

Canary images are published only to Forgejo and GHCR.

## Common checks

- Controller cannot start: verify `config.yaml` exists under `COMPOSIA_CONFIG_DIR` and that required controller paths exist or can be created.
- Agent cannot use Docker: verify `DOCKER_SOCK_GID` matches `/var/run/docker.sock` on the host.
- Web cannot reach controller: `WEB_CONTROLLER_ADDR` is for the web server container, while `WEB_BROWSER_CONTROLLER_ADDR` is for the browser.
