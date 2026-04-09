# Deployment Management

This document explains how to deploy, update, stop, and restart services using Composia.

## Deployment Flow

### 1. Service Discovery

The Controller periodically scans `repo_dir` for directories containing `composia-meta.yaml`:

```
repo/
├── service-a/
│   ├── composia-meta.yaml    ← Discovered
│   └── docker-compose.yaml
├── service-b/
│   ├── composia-meta.yaml    ← Discovered
│   └── docker-compose.yaml
└── README.md
```

### 2. Instance Generation

Each Service generates corresponding ServiceInstances based on the `nodes` configuration:

```yaml
# service-a/composia-meta.yaml
name: service-a
nodes:
  - main
  - edge
```

Generates:
- `service-a` on `main`
- `service-a` on `edge`

### 3. Trigger Deployment

When a user triggers deployment via Web UI or API:

1. Controller validates the service definition
2. Creates `deploy` tasks for each target node
3. Agent retrieves tasks and executes
4. Downloads service bundle (including Compose files and configuration)
5. Renders runtime directory
6. Executes `docker compose up -d`
7. Triggers Caddy sync if `network.caddy` is configured
8. Reports execution result

## Available Operations

### Deploy

First-time deployment of a service to a node.

**Use Cases:**
- Initial deployment of a new service
- First deployment after loading service from Git repository

**Behavior:**
- Downloads service bundle
- Renders runtime directory
- Executes `docker compose up -d`
- Triggers Caddy sync (if `network.caddy` is configured)

### Update

Update an already deployed service.

**Use Cases:**
- Updated `docker-compose.yaml`
- Updated image version
- Updated environment variables

**Behavior:**
- Pulls latest bundle
- Re-renders runtime directory
- Executes `docker compose up -d` (automatically handles changes)
- Triggers Caddy reload

**Notes:**
- Compose automatically determines which containers need rebuilding
- Data volumes are preserved
- Environment variable changes trigger rebuild

### Stop

Stop a service instance.

**Use Cases:**
- Temporarily taking a service offline
- Freeing node resources
- Preparing for service migration

**Behavior:**
- Executes `docker compose down`
- Removes generated Caddy fragment
- Triggers Caddy reload

**Notes:**
- Data volumes are preserved (unless using `down -v`)
- Containers are removed
- Service definition remains in Git repository

### Restart

Restart a service instance.

**Use Cases:**
- Application configuration changes requiring restart
- Temporary issues like memory leaks

**Behavior:**
- Stops and starts sequentially
- Does not re-pull bundle (use Update if needed)

## Using the Web UI

### Deploy a Service

1. Navigate to the **Services** page
2. Click on the target service
3. Find the target node in the **Instances** tab
4. Click the **Deploy** button

### Batch Operations

On the **Services** list page, you can perform batch operations on multiple services:
- Batch deploy
- Batch update
- Batch stop

### View Deployment Status

During deployment, you can view progress in real-time on the **Tasks** page:

| Status | Description |
|--------|-------------|
| `pending` | Waiting to start |
| `running` | Currently executing |
| `awaiting_confirmation` | Waiting for an external confirmation step |
| `succeeded` | Execution successful |
| `failed` | Execution failed |
| `cancelled` | Cancelled |

## Using the API

The current controller exposes ConnectRPC services instead of REST endpoints under `/api/v1/...`.

Use these RPC methods for deployment operations:

- `composia.controller.v1.ServiceCommandService/RunServiceAction` for deploy, update, stop, restart, backup, DNS update, and Caddy sync
- `composia.controller.v1.ServiceCommandService/MigrateService` for migration
- `composia.controller.v1.ServiceInstanceService/RunServiceInstanceAction` for single-instance actions

## Multi-Node Deployment Strategies

### Same Service on Multiple Nodes

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
  - edge-1
  - edge-2
```

Creates instances on all three nodes after deployment.

### Environment Separation

```yaml
# my-app-prod/composia-meta.yaml
name: my-app-prod
nodes:
  - main

---

# my-app-staging/composia-meta.yaml
name: my-app-staging
nodes:
  - edge-1
```

### Rolling Updates

Currently Composia executes updates on all target nodes simultaneously. For rolling updates:

1. First update `nodes` configuration, removing some nodes
2. Wait for update to complete
3. Re-add the nodes
4. Update again

## Deployment Best Practices

### 1. Use Health Checks

```yaml
# docker-compose.yaml
services:
  app:
    image: myapp:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### 2. Configure Restart Policy

```yaml
services:
  app:
    image: myapp:latest
    restart: unless-stopped
```

### 3. Resource Limits

```yaml
services:
  app:
    image: myapp:latest
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
```

### 4. Environment Separation

Use different service names to distinguish environments:

```yaml
# Production
name: my-app-prod
project_name: my-app-prod

# Staging
name: my-app-staging
project_name: my-app-staging
```

### 5. Version Control

Specify explicit versions in image tags:

```yaml
services:
  app:
    image: myapp:1.2.3  # Explicit version
    # Avoid using latest
```

## Troubleshooting

### Deployment Failed

Check task logs:
1. Navigate to the **Tasks** page
2. Find the failed deployment task
3. View detailed logs

Common issues:
- Image pull failure: Check image name and network
- Port conflict: Check port usage
- Missing environment variables: Check `.env` file

### Container Won't Start

On the **Containers** page:
1. Find the target container
2. View logs
3. Check environment variables and volume mounts

### Caddy Configuration Not Applied

Check:
1. Is `network.caddy.enabled` set to `true`?
2. Is `Caddyfile.fragment` path correct?
3. Is Caddy infrastructure service running?

## Related Documentation

- [Service Definition](./service-definition) — How to define services
- [Operations](./operations#task-system) — Understanding task execution
- [Networking](./networking) — Configure reverse proxy
