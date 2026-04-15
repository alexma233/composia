# Agent Configuration

This page documents the `agent` section in `config/config.yaml`.

## Configuration Fields

| Configuration | Type | Required | Description |
|--------------|------|----------|-------------|
| `controller_addr` | string | Yes | Controller API address |
| `node_id` | string | Yes | Node ID, must match the Controller configuration |
| `token` | string | Yes | Node authentication token |
| `repo_dir` | string | Yes | Local service bundle directory |
| `state_dir` | string | Yes | Local runtime state directory |
| `caddy.generated_dir` | string | No | Caddy configuration fragment output directory |

If `agent.controller_addr` starts with `http://`, use it only when TLS is terminated by a trusted reverse proxy or when the controller is reachable only on a trusted local network. Do not send agent tokens over an untrusted cleartext network.

## Same-File Constraints

If one file contains both `controller` and `agent`, these additional rules apply:

- `agent.node_id` must be `main`
- `controller.nodes` must include `main`
- `controller.repo_dir` and `agent.repo_dir` must be different paths

## Minimal Example

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

## Important Path Requirement

If the `agent` talks to the host Docker daemon through `/var/run/docker.sock`, `agent.repo_dir` and `agent.state_dir` must not live only inside an agent-private Docker volume.

These three locations must use the exact same absolute paths:

- `agent.repo_dir` and `agent.state_dir` in `config.yaml`
- the host-side bind mount paths for the `agent` service in `docker-compose.yaml`
- the container-side mount paths for the `agent` service in `docker-compose.yaml`

For the default deployment, keep them aligned like this:

```yaml
agent:
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

```yaml
services:
  agent:
    volumes:
      - /data/repo-agent:/data/repo-agent
      - /data/state-agent:/data/state-agent
```

Do not change the host path to a different location while keeping `/data/...` inside the container, for example:

```yaml
services:
  agent:
    volumes:
      - /srv/composia/repo-agent:/data/repo-agent
```

That makes the agent see `/data/repo-agent`, while the host Docker daemon only sees `/srv/composia/repo-agent`. If a managed service Compose file uses bind mounts such as `/data/repo-agent/...`, the host Docker daemon will resolve `/data/repo-agent/...` on the host and file mounts can fail or turn missing file paths into directories.

## Enable Caddy

Add this to the Agent configuration:

```yaml
agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

You also need to deploy the Caddy infrastructure service. See [Caddy Configuration](../caddy).
