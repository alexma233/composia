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
