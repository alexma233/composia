# Configuration Guide

This document describes the two types of configuration in Composia: platform configuration and service configuration.

## Configuration Types

| Configuration Type | File | Scope | Description |
|-------------------|------|-------|-------------|
| Platform Config | `configs/config.compose.yaml` | Entire platform | Defines how Controller and Agents start |
| Service Config | `composia-meta.yaml` | Individual service | Defines service deployment targets and features |

## Platform Configuration

### Full Configuration Example

```yaml
controller:
  # Network configuration
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  
  # Directory configuration
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  
  # Authentication configuration
  cli_tokens:
    - name: "compose-admin"
      token: "replace-this-token"
      enabled: true
  
  # Node configuration
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
    - id: "edge"
      display_name: "Edge"
      enabled: true
      token: "edge-agent-token"
  
  # Git sync configuration (optional)
  git:
    remote_url: "https://git.example.com/infra/composia.git"
    branch: "main"
    pull_interval: "30s"
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      token_file: "/app/configs/git-token.txt"
  
  # DNS configuration (optional)
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
  
  # Backup configuration (optional)
  rustic:
    main_nodes:
      - "main"
  
  # Secrets configuration (optional)
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-agent-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
  caddy:
    generated_dir: "/srv/caddy/generated"
```

### Controller Configuration

#### Basic Configuration

| Configuration | Type | Required | Description |
|--------------|------|----------|-------------|
| `listen_addr` | string | Yes | Controller listen address, e.g., `:7001` |
| `controller_addr` | string | Yes | Address used by Agents and Web UI to access Controller |
| `repo_dir` | string | Yes | Git working tree directory for storing service definitions |
| `state_dir` | string | Yes | SQLite and runtime state directory |
| `log_dir` | string | Yes | Task logs persistence directory |

#### Authentication Configuration

```yaml
cli_tokens:
  - name: "admin"
    token: "your-secure-token-here"
    enabled: true
  - name: "readonly"
    token: "readonly-token"
    enabled: true
```

| Field | Description |
|-------|-------------|
| `name` | Token name for identification |
| `token` | Token value used by Web UI and CLI |
| `enabled` | Whether this token is enabled |

**Security Recommendations:**
- Use strong random strings as tokens
- Use different tokens for production
- Rotate tokens regularly

#### Node Configuration

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
| `id` | Yes | Unique node identifier; Agent's `node_id` must match |
| `display_name` | No | Display name for Web UI |
| `enabled` | No | Whether to allow this node to connect, default `true` |
| `token` | Yes | Node authentication token |
| `public_ipv4` | No | Node public IPv4 for automatic DNS records |
| `public_ipv6` | No | Node public IPv6 for automatic DNS records |

#### Git Sync Configuration (Optional)

```yaml
git:
  remote_url: "https://github.com/example/composia-services.git"
  branch: "main"
  pull_interval: "30s"
  author_name: "Composia"
  author_email: "composia@example.com"
  auth:
    token_file: "/app/configs/git-token.txt"
```

| Field | Description |
|-------|-------------|
| `remote_url` | Remote Git repository URL |
| `branch` | Branch to track, default `main` |
| `pull_interval` | Auto-pull interval, e.g., `30s`, `5m` |
| `author_name` | Git committer name |
| `author_email` | Git committer email |
| `auth.token_file` | Path to access token file |

#### DNS Configuration (Optional)

```yaml
dns:
  cloudflare:
    api_token_file: "/app/configs/cloudflare-token.txt"
```

#### Secrets Configuration (Optional)

```yaml
secrets:
  provider: age
  identity_file: "/app/configs/age-identity.key"
  recipient_file: "/app/configs/age-recipients.txt"
  armor: true
```

| Field | Description |
|-------|-------------|
| `provider` | Encryption provider, currently only `age` is supported |
| `identity_file` | Path to age private key file |
| `recipient_file` | Path to age public key file |
| `armor` | Whether to use ASCII Armor format |

### Agent Configuration

| Configuration | Type | Required | Description |
|--------------|------|----------|-------------|
| `controller_addr` | string | Yes | Controller API address |
| `node_id` | string | Yes | Node ID, must match Controller configuration |
| `token` | string | Yes | Node authentication token |
| `repo_dir` | string | Yes | Local service bundle directory |
| `state_dir` | string | Yes | Local runtime state directory |
| `caddy.generated_dir` | string | No | Caddy configuration fragment output directory |

## Configuration Recommendations

### Minimal Configuration (Single Node)

```yaml
controller:
  listen_addr: ":7001"
  controller_addr: "http://controller:7001"
  repo_dir: "/data/repo-controller"
  state_dir: "/data/state-controller"
  log_dir: "/data/logs"
  cli_tokens:
    - name: "admin"
      token: "your-token"
      enabled: true
  nodes:
    - id: "main"
      display_name: "Main"
      enabled: true
      token: "main-token"

agent:
  controller_addr: "http://controller:7001"
  node_id: "main"
  token: "main-token"
  repo_dir: "/data/repo-agent"
  state_dir: "/data/state-agent"
```

### Enable Caddy

Add to Agent configuration:

```yaml
agent:
  # ... other configuration
  caddy:
    generated_dir: "/srv/caddy/generated"
```

Also need to deploy the Caddy infrastructure service. See [Networking](./networking).

### Enable Backup

Controller configuration:

```yaml
controller:
  # ... other configuration
  rustic:
    main_nodes:
      - "main"
```

Also need to deploy the rustic infrastructure service. See [Backup & Migration](./backup-migrate).

### Enable DNS

Controller configuration:

```yaml
controller:
  # ... other configuration
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

See [Networking](./networking) for service-side DNS configuration.

### Enable Git Remote Sync

Controller configuration:

```yaml
controller:
  # ... other configuration
  git:
    remote_url: "https://github.com/example/composia-services.git"
    branch: "main"
    pull_interval: "30s"
    auth:
      token_file: "/app/configs/git-token.txt"
```

## Configuration Security

### Token Management

1. **Use read-only mounts for config files**

```yaml
# docker-compose.yaml
volumes:
  - ./configs:/app/configs:ro
```

### Age Key Management

```bash
# Generate age key pair
age-keygen -o key.txt

# Extract public key
cat key.txt | grep "public key" > recipients.txt

# Mount to container
# key.txt as identity_file (private key)
# recipients.txt as recipient_file (public key)
```

## Verify Configuration

Validate configuration by starting the process with the intended config file:

```bash
# Start the Controller with the compose config
go run ./cmd/composia controller -config ./configs/config.compose.yaml

# Start an Agent with the compose config
go run ./cmd/composia agent -config ./configs/config.compose.yaml
```
