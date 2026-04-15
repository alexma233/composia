# Service Definition

Service definitions are at the core of Composia. This document explains how to create and configure services.

## Service Directory Structure

### Minimal Structure

A basic service requires at least two files:

```
my-service/
├── composia-meta.yaml    # Service metadata
└── docker-compose.yaml   # Docker Compose configuration
```

### Full Structure

A fully-featured service directory might include:

```
my-service/
├── composia-meta.yaml      # Service metadata (required)
├── docker-compose.yaml     # Compose configuration (required)
├── .env                    # Environment variables (optional)
├── Caddyfile.fragment      # Caddy configuration fragment (optional)
├── secrets/                # Encrypted secrets (optional)
│   └── database.env.age
└── data/                   # Data directory (optional)
    └── uploads/
```

## composia-meta.yaml

### Full Example

```yaml
# Basic information
name: my-app               # Service unique name (required)
project_name: my-app-prod # Compose project name (optional)
enabled: true              # Whether to enable service declaration (optional, default true)

# Deployment targets
nodes:
  - main
  - edge

# Network configuration
network:
  # Caddy reverse proxy
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  
  # DNS configuration
  dns:
    provider: cloudflare
    hostname: app.example.com
    record_type: A
    proxied: true
    ttl: 120

# Data protection
data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads
    
    - name: database
      backup:
        strategy: database.pgdumpall
        service: postgres

# Backup configuration
backup:
  data:
    - name: uploads
      provider: rustic
    - name: database
      provider: rustic

# Migration configuration
migrate:
  data:
    - name: uploads
    - name: database

# Infrastructure declaration (for infrastructure services only)
infra:
  caddy:
    compose_service: caddy
    config_dir: /etc/caddy
  rustic:
    compose_service: rustic
    profile: default
    data_protect_dir: /data-protect
    init_args:
      - --set-chunker
      - rabin
```

### Field Reference

#### Basic Information

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Service unique identifier, used for URLs and internal references |
| `project_name` | string | No | Override Docker Compose project name |
| `enabled` | boolean | No | Whether to enable service declaration, default `true` |

Composia validates `composia-meta.yaml` in strict mode. Unknown fields are rejected instead of ignored.

#### Deployment Targets

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | array | List of target nodes, each element is a node ID |

**Example:**

```yaml
# Single node deployment
nodes:
  - main

# Multi-node deployment
nodes:
  - main
  - edge-1
  - edge-2
```

#### Network Configuration

**Caddy Configuration (`network.caddy`):**

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | boolean | Whether to enable Caddy reverse proxy |
| `source` | string | Path to Caddyfile fragment, relative to service directory |

**DNS Configuration (`network.dns`):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | DNS provider, currently only `cloudflare` is supported |
| `hostname` | string | Yes | Domain name, e.g., `app.example.com` |
| `record_type` | string | No | Record type: `A`, `AAAA`, or `CNAME`, default `A` |
| `value` | string | No | Record value; auto-derived from node IP if empty |
| `proxied` | boolean | No | Enable Cloudflare proxy, default `false` |
| `ttl` | number | No | TTL in seconds, default `120` |

#### Data Protection

`data_protect` defines data items that can be backed up and restored:

```yaml
data_protect:
  data:
    - name: uploads                    # Data item name
      backup:                          # Backup strategy
        strategy: files.copy           # Backup strategy type
        include:                       # Include paths
          - ./data/uploads
        exclude:                       # Exclude paths (optional)
          - ./data/uploads/temp
      restore:                         # Restore strategy
        strategy: files.copy
        include:
          - ./data/uploads
    
    - name: database
      backup:
        strategy: database.pgdumpall   # PostgreSQL full backup
        service: postgres              # Compose service name
```

**Backup Strategies:**

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `files.copy` | Direct file copy | Static files, upload directories |
| `files.tar_after_stop` | Archive after stopping service | Data requiring consistency |
| `database.pgdumpall` | PostgreSQL full export | PostgreSQL databases |

#### Backup Configuration

`backup` defines which data items participate in backup tasks:

```yaml
backup:
  data:
    - name: uploads
      provider: rustic     # Backup provider
    - name: database
      provider: rustic
```

#### Migration Configuration

`migrate` defines which data is carried over during migration:

```yaml
migrate:
  data:
    - name: uploads
    - name: database
```

**Note:** Data items for migration must have both `backup` and `restore` strategies defined in `data_protect`.

#### Infrastructure Declaration

Used to declare that this service is an infrastructure service (such as Caddy, rustic):

```yaml
infra:
  caddy:
    compose_service: caddy      # Compose service name
    config_dir: /etc/caddy      # Caddy configuration directory
  
  rustic:
    compose_service: rustic     # Compose service name
    profile: default            # rustic profile
    data_protect_dir: /data-protect  # Data protection directory inside the rustic container
    init_args:                  # Extra arguments appended when Settings runs rustic init
      - --set-chunker
      - rabin
```

## docker-compose.yaml

The `docker-compose.yaml` in the service directory is a standard Docker Compose file, fully compatible with Composia.

### Minimal Example

```yaml
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    volumes:
      - ./html:/usr/share/nginx/html
```

### Example with Labels (Recommended)

```yaml
services:
  web:
    image: myapp:latest
    labels:
      - "composia.service=my-app"
      - "traefik.enable=true"
    environment:
      - NODE_ENV=production
    volumes:
      - data:/app/data
    networks:
      - backend

  db:
    image: postgres:15
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: app
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - db_password

volumes:
  data:
  postgres_data:

networks:
  backend:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

### Using Environment Variables

```yaml
services:
  app:
    image: myapp:${APP_VERSION:-latest}
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - LOG_LEVEL=${LOG_LEVEL:-info}
```

`.env` file:

```
APP_VERSION=1.2.3
DATABASE_URL=postgresql://user:pass@db:5432/myapp
LOG_LEVEL=debug
```

## Caddyfile.fragment

When Caddy reverse proxy is enabled, a Caddy configuration fragment needs to be provided:

```caddy
app.example.com {
    reverse_proxy localhost:8080
    
    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
    }
    
    encode gzip
    
    log {
        output file /var/log/caddy/app.log
    }
}
```

**Note:** Caddy fragments don't need the complete Caddyfile structure, just the domain block.

## Service Templates

### Web Application Template

```yaml
# composia-meta.yaml
name: web-app
nodes:
  - main
network:
  caddy:
    enabled: true
    source: ./Caddyfile.fragment
  dns:
    provider: cloudflare
    hostname: app.example.com
    proxied: true
data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads
backup:
  data:
    - name: uploads
      provider: rustic
migrate:
  data:
    - name: uploads
```

```yaml
# docker-compose.yaml
services:
  app:
    image: myapp:latest
    volumes:
      - ./data/uploads:/app/uploads
    environment:
      - NODE_ENV=production
```

### Database Service Template

```yaml
# composia-meta.yaml
name: postgres-main
nodes:
  - main
data_protect:
  data:
    - name: database
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: files.copy
        include:
          - ./restore/
backup:
  data:
    - name: database
      provider: rustic
```

```yaml
# docker-compose.yaml
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: dbuser
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - db_password

volumes:
  postgres_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

## Related Documentation

- [Configuration Guide](./configuration) — Platform configuration reference
- [Deployment](./deployment) — How to deploy services
- [DNS Configuration](./dns) — Detailed DNS configuration
- [Caddy Configuration](./caddy) — Detailed Caddy configuration
- [Backup & Migration](./backup-migrate) — Data protection configuration
