# Controller Configuration

This page documents the `controller` section in `config/config.yaml`.

## Basic Configuration

| Configuration | Type | Required | Description |
|--------------|------|----------|-------------|
| `listen_addr` | string | Yes | Controller listen address, e.g. `:7001` |
| `controller_addr` | string | Yes | Address used by Agents and Web UI to access the Controller |
| `repo_dir` | string | Yes | Git working tree directory for storing service definitions |
| `state_dir` | string | Yes | SQLite and runtime state directory |
| `log_dir` | string | Yes | Task logs persistence directory |
| `nodes` | array | Yes | Must be present even if empty |

## Controller Access Tokens (`access_tokens`)

```yaml
access_tokens:
  - name: "admin"
    token: "your-secure-token-here"
    enabled: true
  - name: "readonly"
    token: "readonly-token"
    enabled: true
```

| Field | Description |
|-------|-------------|
| `name` | Required token name for identification |
| `token` | Required token value used by the Web UI, CLI, or custom clients calling the Controller |
| `enabled` | Whether this token is enabled |
| `comment` | Optional operator-facing note |

Security recommendations:

- Use strong random strings as tokens
- Use different tokens for production
- Rotate tokens regularly

## Node Configuration

```yaml
nodes:
  - id: "main"
    display_name: "Main Server"
    enabled: true
    token: "main-agent-token"
    public_ipv4: "203.0.113.10"
    public_ipv6: "2001:db8::1"
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Unique node identifier; the Agent `node_id` must match |
| `display_name` | No | Display name for the Web UI |
| `enabled` | No | Whether to allow this node to connect, default `true` |
| `token` | Yes | Node authentication token |
| `public_ipv4` | No | Node public IPv4 for automatic DNS records |
| `public_ipv6` | No | Node public IPv6 for automatic DNS records |

`controller.nodes[].id` must be unique.

## Minimal Configuration (Single Node)

```yaml
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  access_tokens:
    - name: "admin"
      token: "your-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-token"
```

To configure the agent side, continue with [Agent Configuration](./agent).
